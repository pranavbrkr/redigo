package server

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestEcho(t *testing.T) {
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

	// ECHO hello
	_, err = conn.Write([]byte("*2\r\n$4\r\nECHO\r\n$5\r\nhello\r\n"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)

	// Expect: $5\r\nhello\r\n
	line1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read line1: %v", err)
	}

	if line1 != "$5\r\n" {
		t.Fatalf("expected $5\\r\\n, got %q", line1)
	}

	line2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read line2: %v", err)
	}
	if line2 != "hello\r\n" {
		t.Fatalf("expected hello\\r\\n, got %q", line2)
	}
}
