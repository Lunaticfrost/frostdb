package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

var (
	// ErrProtocol indicates an invalid RESP syntax error.
	ErrProtocol = errors.New("protocol error")
)

// ReadCommand parses the next command from a RESP2 client connection.
// It supports both standard RESP Array of Bulk Strings (used by redis-cli/SDKs)
// and inline commands (used by telnet/netcat).
func ReadCommand(r *bufio.Reader) ([][]byte, error) {
	b, err := r.Peek(1)
	if err != nil {
		return nil, err
	}

	// 1. Standard RESP Array: *<num_args>\r\n
	if b[0] == '*' {
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}

		count, err := strconv.Atoi(string(line[1:]))
		if err != nil || count <= 0 {
			return nil, fmt.Errorf("%w: invalid array length", ErrProtocol)
		}

		args := make([][]byte, 0, count)
		for i := 0; i < count; i++ {
			arg, err := readBulkString(r)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return args, nil
	}

	// 2. Inline Command: PING\r\n or SET key val\r\n
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}

	parts := bytes.Fields(line)
	if len(parts) == 0 {
		return ReadCommand(r) // Skip empty lines
	}
	return parts, nil
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	n := len(line)
	if n < 2 || line[n-2] != '\r' {
		// Tolerate bare newline without carriage return
		return bytes.TrimRight(line, "\r\n"), nil
	}
	return line[:n-2], nil
}

func readBulkString(r *bufio.Reader) ([]byte, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	if len(line) == 0 || line[0] != '$' {
		return nil, fmt.Errorf("%w: expected '$' for bulk string", ErrProtocol)
	}

	length, err := strconv.Atoi(string(line[1:]))
	if err != nil || length < 0 {
		return nil, fmt.Errorf("%w: invalid bulk string length", ErrProtocol)
	}

	buf := make([]byte, length+2) // +2 for trailing \r\n
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	return buf[:length], nil
}

// WriteSimpleString serializes a RESP simple string (+<string>\r\n).
func WriteSimpleString(w io.Writer, s string) error {
	_, err := fmt.Fprintf(w, "+%s\r\n", s)
	return err
}

// WriteError serializes a RESP error (-ERR <message>\r\n).
func WriteError(w io.Writer, msg string) error {
	_, err := fmt.Fprintf(w, "-ERR %s\r\n", msg)
	return err
}

// WriteInteger serializes a RESP integer (:<int>\r\n).
func WriteInteger(w io.Writer, n int64) error {
	_, err := fmt.Fprintf(w, ":%d\r\n", n)
	return err
}

// WriteBulkString serializes binary data as a RESP bulk string ($<len>\r\n<data>\r\n).
func WriteBulkString(w io.Writer, data []byte) error {
	if data == nil {
		return WriteNull(w)
	}
	header := fmt.Sprintf("$%d\r\n", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\r\n")
	return err
}

// WriteNull serializes a RESP null bulk string ($-1\r\n).
func WriteNull(w io.Writer) error {
	_, err := io.WriteString(w, "$-1\r\n")
	return err
}

// WriteArray serializes an array of bulk strings (*<count>\r\n...).
func WriteArray(w io.Writer, items [][]byte) error {
	header := fmt.Sprintf("*%d\r\n", len(items))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	for _, item := range items {
		if err := WriteBulkString(w, item); err != nil {
			return err
		}
	}
	return nil
}
