package server

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestTTLNonexistentIsMinus2(t *testing.T) {
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

	// TTL missing
	_, err = conn.Write([]byte("*2\r\n$3\r\nTTL\r\n$7\r\nmissing\r\n"))
	if err != nil {
		t.Fatalf("write ttl: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ttl: %v", err)
	}
	if line != ":-2\r\n" {
		t.Fatalf("expected :-2\\r\\n, got %q", line)
	}
}

func TestTTLNoExpiryIsMinus1(t *testing.T) {
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

	// SET a 1
	_, err = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\n1\r\n"))
	if err != nil {
		t.Fatalf("write set: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != "+OK\r\n" {
		t.Fatalf("expected +OK, got %q", line)
	}

	// TTL a => -1
	_, err = conn.Write([]byte("*2\r\n$3\r\nTTL\r\n$1\r\na\r\n"))
	if err != nil {
		t.Fatalf("write ttl: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ttl: %v", err)
	}
	if line != ":-1\r\n" {
		t.Fatalf("expected :-1\\r\\n, got %q", line)
	}
}

func TestExpireThenTTLIsNonNegative(t *testing.T) {
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

	// SET k v
	_, err = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"))
	if err != nil {
		t.Fatalf("write set: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != "+OK\r\n" {
		t.Fatalf("expected +OK, got %q", line)
	}

	// EXPIRE k 2  => :1
	_, err = conn.Write([]byte("*3\r\n$6\r\nEXPIRE\r\n$1\r\nk\r\n$1\r\n2\r\n"))
	if err != nil {
		t.Fatalf("write expire: %v", err)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read expire: %v", err)
	}
	if line != ":1\r\n" {
		t.Fatalf("expected :1\\r\\n, got %q", line)
	}

	// TTL k => should be 0, 1, or 2 depending on timing; but must be >=0
	_, err = conn.Write([]byte("*2\r\n$3\r\nTTL\r\n$1\r\nk\r\n"))
	if err != nil {
		t.Fatalf("write ttl: %v", err)
	}

	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ttl: %v", err)
	}

	// Parse ":<n>\r\n"
	if len(line) < 4 || line[0] != ':' {
		t.Fatalf("expected integer reply, got %q", line)
	}
	// Cheap parse without strconv to keep test tiny
	// We'll just accept it if it's not -1 or -2.
	if line == ":-1\r\n" || line == ":-2\r\n" {
		t.Fatalf("expected non-negative TTL, got %q", line)
	}
}

func TestExpiredKeyBecomesMissing(t *testing.T) {
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

	// SET z 9
	_, err = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\nz\r\n$1\r\n9\r\n"))
	if err != nil {
		t.Fatalf("write set: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != "+OK\r\n" {
		t.Fatalf("expected +OK, got %q", line)
	}

	// EXPIRE z 1
	_, err = conn.Write([]byte("*3\r\n$6\r\nEXPIRE\r\n$1\r\nz\r\n$1\r\n1\r\n"))
	if err != nil {
		t.Fatalf("write expire: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != ":1\r\n" {
		t.Fatalf("expected :1, got %q", line)
	}

	time.Sleep(1200 * time.Millisecond)

	// GET z => null bulk string
	_, err = conn.Write([]byte("*2\r\n$3\r\nGET\r\n$1\r\nz\r\n"))
	if err != nil {
		t.Fatalf("write get: %v", err)
	}
	if line, _ := reader.ReadString('\n'); line != "$-1\r\n" {
		t.Fatalf("expected $-1, got %q", line)
	}
}
