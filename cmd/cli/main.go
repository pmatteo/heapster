package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pmatteo/heapster/internal/config"
	"github.com/pmatteo/heapster/internal/protocol/resp"
)

// Client represents a CLI client for the Heapster server
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	logger *slog.Logger
	parser resp.Parser
}

// NewClient creates a new client connection to the server
func NewClient(addr string, logger *slog.Logger, p resp.Parser) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	logger.Info("connected to server", "addr", addr)

	return &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
		logger: logger,
		parser: p,
	}, nil
}

// SendCommand sends a command to the server and returns the response
func (c *Client) SendCommand(cmd string, args []string) error {
	c.logger.Debug("sending command", "cmd", cmd, "args", args)

	// Encode command as RESP array
	parts := append([]string{cmd}, args...)

	encoded := c.encodeArray(parts)

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		c.logger.Error("failed to set write deadline", "error", err)
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	_, err := c.conn.Write(encoded)
	if err != nil {
		c.logger.Error("failed to send command", "cmd", cmd, "error", err)
		return fmt.Errorf("failed to send command: %w", err)
	}

	return nil
}

// HandleResponse sends a command to the server and returns the response
func (c *Client) HandleResponse() (string, error) {
	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		c.logger.Error("failed to set read deadline", "error", err)
		return "", fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Parse RESP response
	respVal, err := c.parser.Parse(c.reader)
	c.logger.Debug("parsed response value", "respVal", respVal, "error", err)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if respVal == nil {
		c.logger.Info("received nil response from server")
		return "", nil
	}

	// Clear deadlines
	if err := c.conn.SetDeadline(time.Time{}); err != nil {
		c.logger.Error("failed to clear deadline", "error", err)
	}

	return c.formatResponse(respVal), nil
}

// encodeArray encodes a string array as RESP array of bulk strings
func (c *Client) encodeArray(parts []string) []byte {
	bulkStrings := make([][]byte, len(parts))
	for i, part := range parts {
		bulkStrings[i] = resp.EncodeBulkString([]byte(part))
	}
	return resp.EncodeArray(bulkStrings)
}

// formatResponse formats a RespValue for display
func (c *Client) formatResponse(v *resp.RespValue) string {
	if v == nil {
		return ""
	}

	switch v.Type {
	case resp.TypeSimpleString:
		if s, ok := v.AsString(); ok {
			return s
		}
	case resp.TypeError:
		if e, ok := v.AsError(); ok {
			return fmt.Sprintf("ERROR: %s", e.Error())
		}
	case resp.TypeInteger:
		if n, ok := v.AsInt(); ok {
			return fmt.Sprintf("(integer) %d", n)
		}
	case resp.TypeBulkString:
		if bs, ok := v.AsBulkString(); ok {
			return string(bs)
		}
		return "(nil)"
	case resp.TypeArray:
		if arr, ok := v.AsArray(); ok {
			if len(arr) == 0 {
				return "(empty array)"
			}
			var sb strings.Builder
			for i, elem := range arr {
				sb.WriteString(fmt.Sprintf("%d) %s", i+1, c.formatResponse(elem)))
				if i < len(arr)-1 {
					sb.WriteString("\n")
				}
			}
			return sb.String()
		}
		return "(nil)"
	}
	return "(unknown)"
}

// Close closes the client connection
func (c *Client) Close() error {
	c.logger.Info("closing client connection")
	return c.conn.Close()
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	addr := flag.String("addr", "", "server address (overrides config)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelError,
	}))

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Use addr flag if provided, otherwise use config
	serverAddr := cfg.Server.Addr
	if *addr != "" {
		serverAddr = *addr
	}

	logger.Info("loaded config", "config_path", *configPath, "server_addr", serverAddr)

	// create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	client, err := NewClient(serverAddr, logger, resp.NewParserAdapter(logger))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	logger.Info("client started", "addr", serverAddr)
	fmt.Println("\n\nType commands (e.g., 'SADD key value', 'SMEMBERS', 'QUIT' to exit)")
	fmt.Println("Press Ctrl+C for graceful shutdown")
	fmt.Println()

	// Channel to signal when input loop is done
	done := make(chan struct{})

	// Goroutine to handle user input
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(os.Stdin)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			fmt.Print("heapster> ")

			// create channel for scanner result
			inputChan := make(chan string, 1)
			go func() {
				if scanner.Scan() {
					inputChan <- scanner.Text()
				} else {
					close(inputChan)
				}
			}()

			// Wait for input or context cancellation
			select {
			case <-ctx.Done():
				return
			case input, ok := <-inputChan:
				if !ok {
					logger.Debug("scanner finished")
					return
				}

				input = strings.TrimSpace(input)
				if input == "" {
					continue
				}

				// Handle QUIT command locally
				if strings.ToUpper(input) == "QUIT" || strings.ToUpper(input) == "EXIT" {
					logger.Info("quit command received")
					cancel()
					return
				}

				// Parse command and arguments
				parts := strings.Fields(input)
				if len(parts) == 0 {
					continue
				}

				cmd := strings.ToUpper(parts[0])
				args := parts[1:]

				// Send command to server
				if err := client.SendCommand(cmd, args); err != nil {
					logger.Info("send command error", "err", err)
					continue
				}

				response, err := client.HandleResponse()
				if err != nil {
					logger.Info("error reading response", "err", err)
					continue
				}

				fmt.Println(response)
			}
		}
	}()

	// Wait for either signal or input completion
	select {
	case <-sigChan:
		logger.Info("received interrupt signal, shutting down gracefully")
		cancel()
	case <-done:
		logger.Debug("input loop completed")
	}

	// Wait for goroutine to finish
	<-done

	logger.Info("client shutdown complete")
}
