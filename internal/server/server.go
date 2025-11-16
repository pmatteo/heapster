package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/pmatteo/heapster/internal/protocol/resp"
	"github.com/pmatteo/heapster/internal/store"
)

// RespServer is the public server interface; NewRespServer returns an implementation.
type RespServer interface {
	Start(ctx context.Context, addr string) error
	Stop(ctx context.Context) error
}

type respServer struct {
	store   store.Store
	parser  resp.Parser
	encoder resp.Encoder
	logger  *slog.Logger

	handler HandlerFunc

	mu       sync.Mutex
	listener net.Listener
	wg       sync.WaitGroup
	closing  chan struct{}
}

// Start begins listening on the specified address and handles incoming connections
func (s *respServer) Start(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.logger.Error("failed to listen", "addr", addr, "error", err)
		return err
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	s.logger.Info("server started", "addr", addr)

	acceptErrCh := make(chan error, 1)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-s.closing:
					acceptErrCh <- nil
					return
				default:
					s.logger.Error("accept error", "error", err)
					acceptErrCh <- err
					return
				}
			}

			s.logger.Debug("accepted connection", "remote_addr", conn.RemoteAddr())

			s.wg.Add(1)
			go func(c net.Conn) {
				defer s.wg.Done()
				s.handleConn(c)
			}(conn)
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("server context cancelled")
		_ = s.Stop(context.Background())
		return ctx.Err()
	case err := <-acceptErrCh:
		_ = s.Stop(context.Background())
		return err
	}
}

// Stop gracefully shuts down the server
func (s *respServer) Stop(ctx context.Context) error {
	s.logger.Info("stopping server")

	s.mu.Lock()
	if s.listener == nil {
		s.mu.Unlock()
		return nil
	}
	_ = s.listener.Close()

	// Only close once
	select {
	case <-s.closing:
		// Already closed
	default:
		close(s.closing)
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("server stopped")
		return nil
	case <-ctx.Done():
		s.logger.Warn("server stop context cancelled")
		return ctx.Err()
	}
}

// handleConn processes commands from a single client connection
func (s *respServer) handleConn(conn net.Conn) {
	defer conn.Close()

	// Recover from panics to prevent server crash
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			s.logger.Error("panic in connection handler",
				"remote_addr", conn.RemoteAddr(),
				"panic", r,
				"stack", stack)
		}
	}()

	r := bufio.NewReader(conn)

	for {
		msg, err := s.parser.Parse(r)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
				s.logger.Info("connection closed")
				return
			}
			s.logger.Error("parser error", "error", err)
			_ = s.encoder.Encode(conn, fmt.Errorf("ERR parsing message: %v", err))
			return
		}

		if msg == nil {
			s.logger.Info("received nil message (likely EOF)")
			return
		}

		arr, ok := msg.AsArray()
		if !ok {
			s.logger.Error("message is not an array", "type", msg.Type)
			if err := s.encoder.Encode(conn, errors.New("ERR expected array")); err != nil {
				s.logger.Error("unable to encode error", "error", err)
			}
			continue
		}

		if len(arr) == 0 {
			s.logger.Error("empty array received")
			if err := s.encoder.Encode(conn, errors.New("ERR empty command")); err != nil {
				s.logger.Error("unable to encode error", "error", err)
			}
			continue
		}

		// Extract command - should be a bulk string or simple string
		var cmd string
		if bulkStr, ok := arr[0].AsBulkString(); ok {
			cmd = string(bulkStr)
		} else if simpleStr, ok := arr[0].AsString(); ok {
			cmd = simpleStr
		} else {
			s.logger.Error("invalid command type", "type", arr[0].Type)
			if err := s.encoder.Encode(conn, errors.New("ERR invalid command type")); err != nil {
				s.logger.Error("unable to encode error", "error", err)
			}
			continue
		}

		// Extract arguments
		args := make([]string, 0, len(arr)-1)
		for i, vt := range arr[1:] {
			var arg string
			if bulkStr, ok := vt.AsBulkString(); ok {
				arg = string(bulkStr)
			} else if simpleStr, ok := vt.AsString(); ok {
				arg = simpleStr
			} else {
				s.logger.Error("invalid argument type", "index", i+1, "type", vt.Type)
				if err := s.encoder.Encode(conn, fmt.Errorf("ERR invalid argument type at position %d", i+1)); err != nil {
					s.logger.Error("unable to encode error", "error", err)
				}
				continue
			}
			args = append(args, arg)
		}

		// Check if handler is set
		if s.handler == nil {
			s.logger.Error("handler is nil")
			if err := s.encoder.Encode(conn, errors.New("ERR server not properly configured")); err != nil {
				s.logger.Error("unable to encode error", "error", err)
			}
			continue
		}

		req := &Request{
			Cmd:  strings.ToUpper(cmd),
			Args: args,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := s.handler(ctx, req)
		s.logger.Debug("handled command", "resp", resp, "err", err)

		if err != nil {
			s.logger.Error("handler error", "cmd", cmd, "error", err)
			if err = s.encoder.Encode(conn, err); err != nil {
				s.logger.Error("unable to encode error", "error", err)
			}
			continue
		}

		s.logger.Debug("encoding response", "resp", resp)
		if err = s.encoder.Encode(conn, resp); err != nil {
			s.logger.Error("unable to encode response", "error", err)
		}
	}
}
