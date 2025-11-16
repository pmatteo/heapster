package resp

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

type Encoder interface {
	Encode(w io.Writer, v any) error
}

// encoderAdapter provides a flexible type-based encoding mechanism.
type encoderAdapter struct {
	handlers map[string]func(any) ([]byte, error)
}

// NewEncoderAdapter creates a new encoder adapter with default handlers registered.
func NewEncoderAdapter() *encoderAdapter {
	e := &encoderAdapter{
		handlers: make(map[string]func(any) ([]byte, error)),
	}

	// Register default type handlers
	e.Register((error)(nil), func(v any) ([]byte, error) {
		return EncodeError(v.(error).Error()), nil
	})
	e.Register((int64)(0), func(v any) ([]byte, error) {
		return EncodeInteger(v.(int64)), nil
	})
	e.Register("", func(v any) ([]byte, error) {
		return EncodeSimpleString(v.(string)), nil
	})
	e.Register(([]string)(nil), func(v any) ([]byte, error) {
		val := v.([]string)
		elems := make([][]byte, len(val))
		for i, s := range val {
			elems[i] = EncodeBulkString([]byte(s))
		}
		return EncodeArray(elems), nil
	})

	return e
}

// Register allows adding new custom handlers for specific types.
func (e *encoderAdapter) Register(example any, fn func(any) ([]byte, error)) {
	e.handlers[fmt.Sprintf("%T", example)] = fn
}

// Encode writes the RESP-encoded representation of v to w.
// It uses registered handlers or returns an error for unhandled types.
func (e *encoderAdapter) Encode(w io.Writer, v any) error {
	if v == nil {
		_, err := w.Write([]byte("+OK\r\n"))
		return err
	}

	// Check for error interface first
	if err, ok := v.(error); ok {
		data := EncodeError(err.Error())
		if _, writeErr := w.Write(data); writeErr != nil {
			return fmt.Errorf("encode write failed: %w", writeErr)
		}
		return nil
	}

	typeName := fmt.Sprintf("%T", v)
	fn, ok := e.handlers[typeName]
	if !ok {
		return fmt.Errorf("no encoder registered for type %T", v)
	}

	data, err := fn(v)
	if err != nil {
		return fmt.Errorf("encode failed: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("encode write failed: %w", err)
	}

	return nil
}

// EncodeSimpleString encodes a simple string ("+OK\r\n").
func EncodeSimpleString(s string) []byte {
	return []byte("+" + s + "\r\n")
}

// EncodeError encodes an error string ("-ERR msg\r\n").
func EncodeError(s string) []byte {
	return []byte("-" + s + "\r\n")
}

// EncodeInteger encodes an integer (":1000\r\n").
func EncodeInteger(n int64) []byte {
	return []byte(":" + strconv.FormatInt(n, 10) + "\r\n")
}

// EncodeBulkString encodes a bulk string. If bs is nil it encodes $-1\r\n.
func EncodeBulkString(bs []byte) []byte {
	if bs == nil {
		return []byte("$-1\r\n")
	}
	return []byte("$" + strconv.Itoa(len(bs)) + "\r\n" + string(bs) + "\r\n")
}

// EncodeArray encodes an array of pre-encoded RESP elements.
// Each element should already be a RESP-encoded byte slice.
func EncodeArray(elems [][]byte) []byte {
	if elems == nil {
		return []byte("*-1\r\n")
	}
	b := &bytes.Buffer{}
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(elems)))
	b.WriteString("\r\n")
	for _, e := range elems {
		b.Write(e)
	}
	return b.Bytes()
}
