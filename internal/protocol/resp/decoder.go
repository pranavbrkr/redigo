package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Decode reads a single RESP2 value from the reader
func Decode(r *bufio.Reader) (Value, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch prefix {

	// Simple string
	case '+':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: SimpleString, Str: line}, nil

	// Error
	case '-':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: Error, Str: line}, nil

	// Integer
	case ':':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}
		n, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return Value{}, ProtoError{Msg: "invalid integer"}
		}
		return Value{Type: Integer, Int: n}, nil

	// Bulk String
	case '$':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}

		size, err := strconv.Atoi(line)
		if err != nil {
			return Value{}, ProtoError{Msg: "invalid bulk length"}
		}

		if size == -1 {
			return Value{Type: BulkString, Bulk: nil}, nil
		}

		buf := make([]byte, size)
		if _, err := io.ReadFull(r, buf); err != nil {
			return Value{}, err
		}

		// consume trailing \r\n
		if _, err := r.ReadString('\n'); err != nil {
			return Value{}, err
		}
		return Value{Type: BulkString, Bulk: buf}, nil

	// Array
	case '*':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}

		count, err := strconv.Atoi(line)
		if err != nil {
			return Value{}, ProtoError{Msg: "invalid array length"}
		}

		if count == -1 {
			return Value{Type: Array, Array: nil}, nil
		}

		items := make([]Value, 0, count)
		for i := 0; i < count; i++ {
			v, err := Decode(r)
			if err != nil {
				return Value{}, err
			}
			items = append(items, v)
		}
		return Value{Type: Array, Array: items}, nil

	default:
		return Value{}, ProtoError{Msg: fmt.Sprintf("unknown RESP prefixL %q", prefix)}
	}
}

// readLine reads a CRLF-terminated line and strips \r\n
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSuffix(line, "\r\n")
	return line, nil
}
