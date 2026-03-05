package server

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/store"
)

func TestUnknownCommand_ReturnsError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_, _ = conn.Write([]byte("*1\r\n$4\r\nFAKE\r\n"))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error reply, got %q", line)
	}
	if !strings.Contains(line, "unknown command") {
		t.Fatalf("expected unknown command in error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "fake") {
		t.Fatalf("expected command name in error, got %q", line)
	}
}

func TestWrongArgs_SET_ReturnsError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// SET with no args: *2\r\n$3\r\nSET\r\n
	_, _ = conn.Write([]byte("*1\r\n$3\r\nSET\r\n"))
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "wrong number of arguments") {
		t.Fatalf("expected wrong number of arguments, got %q", line)
	}
}

func TestWrongArgs_GET_ReturnsError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// GET with 0 args
	_, _ = conn.Write([]byte("*1\r\n$3\r\nGET\r\n"))
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "wrong number of arguments") {
		t.Fatalf("expected wrong number of arguments, got %q", line)
	}
}

func TestWrongArgs_ECHO_ReturnsError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// ECHO with 0 args
	_, _ = conn.Write([]byte("*1\r\n$4\r\nECHO\r\n"))
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "wrong number of arguments") {
		t.Fatalf("expected wrong number of arguments, got %q", line)
	}
}

func TestEXPIRE_NonInteger_ReturnsError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
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

	writeCommand(conn, "SET", "k", "v")
	_, _ = reader.ReadString('\n')

	writeCommand(conn, "EXPIRE", "k", "notanumber")
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "integer") {
		t.Fatalf("expected integer-related error, got %q", line)
	}
}

func TestEXPIREAT_NonInteger_ReturnsError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
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

	writeCommand(conn, "SET", "k", "v")
	_, _ = reader.ReadString('\n')

	writeCommand(conn, "EXPIREAT", "k", "bad")
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "integer") {
		t.Fatalf("expected integer-related error, got %q", line)
	}
}

func TestNonArray_ReturnsExpectedArrayError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a single bulk string instead of array
	_, _ = conn.Write([]byte("$3\r\nGET\r\n"))
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "expected array") {
		t.Fatalf("expected array of bulk strings message, got %q", line)
	}
}

func TestEmptyArray_ReturnsExpectedArrayError(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_, _ = conn.Write([]byte("*0\r\n"))
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "expected array") {
		t.Fatalf("expected array of bulk strings message, got %q", line)
	}
}

func TestINFO_WithArgs_ReturnsWrongArgs(t *testing.T) {
	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, nil, aof.FsyncEverySecond)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_, _ = conn.Write([]byte("*2\r\n$4\r\nINFO\r\n$3\r\nfoo\r\n"))
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "wrong number of arguments") {
		t.Fatalf("expected wrong number of arguments, got %q", line)
	}
}
