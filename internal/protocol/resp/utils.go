package resp

import (
	"bufio"
	"errors"
)

// readLine reads bytes until CRLF and returns the bytes before CRLF.
func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes(LF)
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != CR {
		return nil, errors.New("line not terminated with CRLF")
	}
	return line[:len(line)-2], nil
}
