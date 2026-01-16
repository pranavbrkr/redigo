package server

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestSetThenGet(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// SET foo bar
	_, err = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"))
	if err != nil {
		t.Fatalf("write ser: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read set: %v", err)
	}
	if line != "+OK\r\n" {
		t.Fatalf("expected +OK\\r\\n, got %q", line)
	}

	// GET foo
	_, err = conn.Write([]byte("*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"))
	if err != nil {
		t.Fatalf("write get: %v", err)
	}

	// Expect: $3\r\nbar\r\n
	line1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read get line1: %v", err)
	}
	if line1 != "$3\r\n" {
		t.Fatalf("expected $3\\r\\n, got %q", line1)
	}

	line2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read get line2: %v", err)
	}
	if line2 != "bar\r\n" {
		t.Fatalf("expected bar \\r\\n, got %q", line2)
	}
}

func TestGetMissingReturnsNullBulkString(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// GET missing
	_, err = conn.Write([]byte("*2\r\n$3\r\nGET\r\n$7\r\nmissing\r\n"))
	if err != nil {
		t.Fatalf("write get: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if line != "$-1\r\n" {
		t.Fatalf("expected $-1\\r\\n, got %q", line)
	}
}
