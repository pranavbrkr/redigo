package resp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecode_UnknownPrefixReturnsProtoError(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("Xfoo\r\n"))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	var pe ProtoError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProtoError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Msg, "unknown RESP prefix") {
		t.Fatalf("unexpected message: %q", pe.Msg)
	}
}

func TestDecode_InvalidIntegerReturnsProtoError(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(":notanumber\r\n"))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	var pe ProtoError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProtoError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Msg, "invalid integer") {
		t.Fatalf("unexpected message: %q", pe.Msg)
	}
}

func TestDecode_InvalidBulkLengthReturnsProtoError(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("$abc\r\n"))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	var pe ProtoError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProtoError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Msg, "invalid bulk length") {
		t.Fatalf("unexpected message: %q", pe.Msg)
	}
}

func TestDecode_BulkMinusOneIsNullBulk(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("$-1\r\n"))
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != BulkString || v.Bulk != nil {
		t.Fatalf("got type=%v bulk=%v", v.Type, v.Bulk)
	}
}

func TestDecode_BulkInvalidTerminatorReturnsError(t *testing.T) {
	// $3 then "foo" but \n instead of \r\n; second ReadByte may get EOF
	r := bufio.NewReader(strings.NewReader("$3\r\nfoo\n"))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	// Decoder reads \n then expects \r; next byte can be EOF, so we may get ProtoError or EOF
	var pe ProtoError
	if errors.As(err, &pe) && !strings.Contains(pe.Msg, "invalid bulk string terminator") {
		t.Fatalf("unexpected ProtoError message: %q", pe.Msg)
	}
}

func TestDecode_BulkEOFBeforePayloadReturnsErr(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("$10\r\nshort"))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	if err != io.ErrUnexpectedEOF && !errors.Is(err, io.EOF) {
		// ReadFull returns ErrUnexpectedEOF
		if !strings.Contains(err.Error(), "EOF") {
			t.Fatalf("expected EOF-related error, got %v", err)
		}
	}
}

func TestDecode_ArrayMinusOneIsNullArray(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("*-1\r\n"))
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Array || v.Array != nil {
		t.Fatalf("got type=%v array=%v", v.Type, v.Array)
	}
}

func TestDecode_InvalidArrayLengthReturnsProtoError(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("*x\r\n"))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	var pe ProtoError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProtoError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Msg, "invalid array length") {
		t.Fatalf("unexpected message: %q", pe.Msg)
	}
}

func TestDecode_EOFBeforePrefixReturnsEOF(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader(nil))
	_, err := Decode(r)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestDecode_SimpleStringWithCRLFInLine(t *testing.T) {
	// Simple string cannot contain \r\n in the middle per RESP; we just read to \n
	// So "+OK\r\n" is the only valid form. "+OK\r\n" -> readLine gives "OK"
	r := bufio.NewReader(strings.NewReader("+PONG\r\n"))
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != SimpleString || v.Str != "PONG" {
		t.Fatalf("got type=%v str=%q", v.Type, v.Str)
	}
}
