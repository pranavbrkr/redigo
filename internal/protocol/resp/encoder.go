package resp

import (
	"bufio"
	"strconv"
)

// Writes RESP simple string: +<text>\r\n
func WriteSimpleString(w *bufio.Writer, s string) error {
	if _, err := w.WriteString("+"); err != nil {
		return err
	}
	if _, err := w.WriteString(s); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

// Writes RESP error: -<text>\r\n
func WriteError(w *bufio.Writer, msg string) error {
	if _, err := w.WriteString("-"); err != nil {
		return err
	}
	if _, err := w.WriteString(msg); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

// Writes RESP Integer: <n>\r\n
func WriteInteger(w *bufio.Writer, n int64) error {
	if _, err := w.WriteString(":"); err != nil {
		return err
	}
	if _, err := w.WriteString(strconv.FormatInt(n, 10)); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

// Writes RESP Bulk String: $<len>\r\n<bytes>\r\n
// If b is nil, writes Null Bulk String: $-1\r\n
func WriteBulkString(w *bufio.Writer, b []byte) error {
	if b == nil {
		_, err := w.WriteString("$-1\r\n")
		return err
	}

	if _, err := w.WriteString("$"); err != nil {
		return err
	}
	if _, err := w.WriteString(strconv.Itoa(len(b))); err != nil {
		return err
	}
	if _, err := w.WriteString("\r\n"); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

// WriteArrayHeader writes a RESP Array header: *<n>\r\n
func WriteArrayHeader(w *bufio.Writer, n int) error {
	if _, err := w.WriteString("*"); err != nil {
		return err
	}
	if _, err := w.WriteString(strconv.Itoa(n)); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

// WriteNullArray writes a Null Array: *-1\r\n
func WriteNullArray(w *bufio.Writer) error {
	_, err := w.WriteString("*-1\r\n")
	return err
}
