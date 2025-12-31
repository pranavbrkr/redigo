package server

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestExists(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st)
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

	// SET a 1
	_, err = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\n1\r\n"))
	if err != nil {
		t.Fatalf("write set: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != "+OK\r\n" {
		t.Fatalf("expected +OK, got %q", line)
	}

	// EXISTS a, b
	_, err = conn.Write([]byte("*3\r\n$6\r\nEXISTS\r\n$1\r\na\r\n$1\r\nb\r\n"))
	if err != nil {
		t.Fatalf("write exists: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read exists: %v", err)
	}
	if line != ":1\r\n" {
		t.Fatalf("expected :1\\r\\n, got %q", line)
	}
}

func TestDel(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st)
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
		t.Fatalf("write set: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != "+OK\r\n" {
		t.Fatalf("expected +OK, got %q", line)
	}

	// DEL foo missing
	_, err = conn.Write([]byte("*3\r\n$3\r\nDEL\r\n$3\r\nfoo\r\n$7\r\nmissing\r\n"))
	if err != nil {
		t.Fatalf("write del: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read del: %v", err)
	}
	if line != ":1\r\n" {
		t.Fatalf("expected :1\\r\\n, got %q", line)
	}

	// GET foo: should be null bulk string now
	_, err = conn.Write([]byte("*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"))
	if err != nil {
		t.Fatalf("write get: %v", err)
	}

	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read get: %v", err)
	}
	if line != "$-1\r\n" {
		t.Fatalf("expected $-1\\r\\n, got %q", line)
	}
}
