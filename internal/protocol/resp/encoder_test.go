package resp

import (
	"bufio"
	"bytes"
	"testing"
)

func TestRoundtrip_SimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteSimpleString(w, "OK"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != SimpleString || v.Str != "OK" {
		t.Fatalf("got type=%v str=%q", v.Type, v.Str)
	}
}

func TestRoundtrip_SimpleStringEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteSimpleString(w, ""); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != SimpleString || v.Str != "" {
		t.Fatalf("got type=%v str=%q", v.Type, v.Str)
	}
}

func TestRoundtrip_Error(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteError(w, "ERR something"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Error || v.Str != "ERR something" {
		t.Fatalf("got type=%v str=%q", v.Type, v.Str)
	}
}

func TestRoundtrip_Integer(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteInteger(w, 42); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Integer || v.Int != 42 {
		t.Fatalf("got type=%v int=%d", v.Type, v.Int)
	}
}

func TestRoundtrip_IntegerNegative(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteInteger(w, -2); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Integer || v.Int != -2 {
		t.Fatalf("got type=%v int=%d", v.Type, v.Int)
	}
}

func TestRoundtrip_BulkString(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteBulkString(w, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != BulkString || string(v.Bulk) != "hello" {
		t.Fatalf("got type=%v bulk=%q", v.Type, v.Bulk)
	}
}

func TestRoundtrip_NullBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteBulkString(w, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != BulkString || v.Bulk != nil {
		t.Fatalf("got type=%v bulk=%v", v.Type, v.Bulk)
	}
}

func TestRoundtrip_Array(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteArrayHeader(w, 2); err != nil {
		t.Fatal(err)
	}
	if err := WriteBulkString(w, []byte("GET")); err != nil {
		t.Fatal(err)
	}
	if err := WriteBulkString(w, []byte("foo")); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Array || len(v.Array) != 2 {
		t.Fatalf("got type=%v len=%d", v.Type, len(v.Array))
	}
	if v.Array[0].Type != BulkString || string(v.Array[0].Bulk) != "GET" {
		t.Fatalf("got [0]=%v %q", v.Array[0].Type, v.Array[0].Bulk)
	}
	if v.Array[1].Type != BulkString || string(v.Array[1].Bulk) != "foo" {
		t.Fatalf("got [1]=%v %q", v.Array[1].Type, v.Array[1].Bulk)
	}
}

func TestRoundtrip_NullArray(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteNullArray(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Array || v.Array != nil {
		t.Fatalf("got type=%v array=%v", v.Type, v.Array)
	}
}

func TestRoundtrip_EmptyArray(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := WriteArrayHeader(w, 0); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(&buf)
	v, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Array || len(v.Array) != 0 {
		t.Fatalf("got type=%v len=%d", v.Type, len(v.Array))
	}
}
