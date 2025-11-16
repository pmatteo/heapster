package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
)

// ErrNilBulkString is returned when a bulk string is RESP Null Bulk String ($-1).
var ErrNilBulkString = errors.New("nil bulk string")

// ErrNilArray is returned when an array is RESP Null Array (*-1).
var ErrNilArray = errors.New("nil array")

// Parser defines the RESP parsing interface.
type Parser interface {
	Parse(r *bufio.Reader) (*RespValue, error)
}

// parserAdapter provides a flexible RESP parsing interface with logging.
type parserAdapter struct {
	handlers map[byte]func(*parserAdapter, *bufio.Reader) (*RespValue, error)
	logger   *slog.Logger
}

// NewParserAdapter creates a parser with default RESP type handlers and optional logger.
func NewParserAdapter(l *slog.Logger) *parserAdapter {
	if l == nil {
		l = slog.Default()
	}

	p := &parserAdapter{
		handlers: make(map[byte]func(*parserAdapter, *bufio.Reader) (*RespValue, error)),
		logger:   l,
	}

	p.Register(SimpleStringPrefix, func(_ *parserAdapter, r *bufio.Reader) (*RespValue, error) {
		val, err := ParseSimpleString(r)
		if err != nil {
			return nil, fmt.Errorf("parse simple string: %w", err)
		}
		return &RespValue{Type: TypeSimpleString, Val: val}, nil
	})

	p.Register(ErrorPrefix, func(_ *parserAdapter, r *bufio.Reader) (*RespValue, error) {
		val, err := ParseError(r)
		if err != nil {
			return nil, fmt.Errorf("parse error: %w", err)
		}
		return &RespValue{Type: TypeError, Val: val}, nil
	})

	p.Register(IntegerPrefix, func(_ *parserAdapter, r *bufio.Reader) (*RespValue, error) {
		val, err := ParseInteger(r)
		if err != nil {
			return nil, fmt.Errorf("parse integer: %w", err)
		}
		return &RespValue{Type: TypeInteger, Val: val}, nil
	})

	p.Register(BulkStringPrefix, func(_ *parserAdapter, r *bufio.Reader) (*RespValue, error) {
		val, err := ParseBulkString(r)
		if err != nil {
			return nil, fmt.Errorf("parse bulk string: %w", err)
		}
		return &RespValue{Type: TypeBulkString, Val: val}, nil
	})

	// Array handler uses recursion through same parser instance.
	p.Register(ArrayPrefix, func(p *parserAdapter, r *bufio.Reader) (*RespValue, error) {
		n, err := ParseArrayLength(r)
		if err != nil {
			p.logger.Error("failed to parse array length", "error", err)
			return nil, fmt.Errorf("parse array length: %w", err)
		}

		if n < 0 {
			p.logger.Info("parsed nil array")
			return &RespValue{Type: TypeArray, Val: nil}, nil
		}

		arr := make([]*RespValue, n)
		for i := 0; i < n; i++ {
			elem, err := p.Parse(r)
			if err != nil {
				p.logger.Error("failed to parse array element", "index", i, "error", err)
				return nil, fmt.Errorf("parse array element %d: %w", i, err)
			}
			arr[i] = elem
		}

		return &RespValue{Type: TypeArray, Val: arr}, nil
	})

	return p
}

// Register allows adding new RESP type handlers dynamically.
func (p *parserAdapter) Register(prefix byte, fn func(*parserAdapter, *bufio.Reader) (*RespValue, error)) {
	p.handlers[prefix] = fn
}

// Parse reads a single RESP value from r and returns it as a RespValue.
func (p *parserAdapter) Parse(r *bufio.Reader) (*RespValue, error) {
	b, err := r.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		p.logger.Error("failed to read RESP type byte", "error", err)
		return nil, fmt.Errorf("read type byte: %w", err)
	}

	p.logger.Debug("parsing RESP value", "firstByte", string(b))

	handler, ok := p.handlers[b]
	if !ok {
		p.logger.Error("unknown RESP type byte", "byte", string(b))
		return nil, fmt.Errorf("unknown RESP type byte: %q", b)
	}

	val, err := handler(p, r)
	if err != nil {
		p.logger.Error("handler failed", "type", string(b), "error", err)
		return nil, fmt.Errorf("parse RESP value (type=%q): %w", b, err)
	}

	return val, nil
}

// ParseSimpleString parses a RESP simple string into Go string.
func ParseSimpleString(r *bufio.Reader) (string, error) {
	line, err := readLine(r)
	if err != nil {
		return "", err
	}
	return string(line), nil
}

// ParseError parses a RESP error into Go error.
func ParseError(r *bufio.Reader) (error, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	return errors.New(string(line)), nil
}

// ParseInteger parses a RESP integer into int64.
func ParseInteger(r *bufio.Reader) (int64, error) {
	line, err := readLine(r)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(line), 10, 64)
}

// ParseBulkString parses a RESP bulk string into []byte.
func ParseBulkString(r *bufio.Reader) ([]byte, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	length, err := strconv.Atoi(string(line))
	if err != nil {
		return nil, err
	}
	if length == -1 {
		return nil, ErrNilBulkString
	}

	buf := make([]byte, length+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if buf[length] != CR || buf[length+1] != LF {
		return nil, errors.New("bulk string not terminated with CRLF")
	}
	return buf[:length], nil
}

// ParseArrayLength reads the length of a RESP array (e.g., "*3\r\n" → 3).
func ParseArrayLength(r *bufio.Reader) (int, error) {
	line, err := readLine(r)
	if err != nil {
		return 0, fmt.Errorf("failed to read array length: %w", err)
	}
	length, err := strconv.Atoi(string(line))
	if err != nil {
		return 0, fmt.Errorf("invalid array length: %q", line)
	}
	return length, nil
}
