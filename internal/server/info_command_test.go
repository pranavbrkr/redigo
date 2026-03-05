package server

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/protocol/resp"
	"github.com/pranavbrkr/redigo/internal/store"
)

func TestINFO_ReturnsBulkWithServerSection(t *testing.T) {
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

	// INFO
	_, _ = conn.Write([]byte("*1\r\n$4\r\nINFO\r\n"))
	reader := bufio.NewReader(conn)

	v, err := resp.Decode(reader)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v.Type != resp.BulkString || v.Bulk == nil {
		t.Fatalf("expected bulk response, got type=%v", v.Type)
	}
	body := string(v.Bulk)
	if !strings.Contains(body, "# Server") {
		t.Fatalf("expected # Server in INFO, got %q", body)
	}
	if !strings.Contains(body, "redis_version:") {
		t.Fatalf("expected redis_version in INFO, got %q", body)
	}
	if !strings.Contains(body, "redigo:1") {
		t.Fatalf("expected redigo:1 in INFO, got %q", body)
	}
	if s.ln != nil {
		if a, ok := s.ln.Addr().(*net.TCPAddr); ok {
			portStr := strconv.Itoa(a.Port)
			if !strings.Contains(body, "tcp_port:"+portStr) {
				t.Fatalf("expected tcp_port:%s in INFO, got %q", portStr, body)
			}
		}
	}
}

func TestCOMMAND_NoArgs_ReturnsArrayOfCommandDocs(t *testing.T) {
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

	_, _ = conn.Write([]byte("*1\r\n$7\r\nCOMMAND\r\n"))
	reader := bufio.NewReader(conn)

	v, err := resp.Decode(reader)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v.Type != resp.Array || v.Array == nil {
		t.Fatalf("expected array, got type=%v", v.Type)
	}
	if len(v.Array) != 11 {
		t.Fatalf("expected 11 command docs, got %d", len(v.Array))
	}
	// First doc should be PING (name, arity, flags)
	pingDoc := v.Array[0]
	if pingDoc.Type != resp.Array || len(pingDoc.Array) < 2 {
		t.Fatalf("expected command doc array with at least name and arity, got %v", pingDoc.Type)
	}
	if pingDoc.Array[0].Type != resp.BulkString || string(pingDoc.Array[0].Bulk) != "ping" {
		t.Fatalf("expected first command name ping, got %q", pingDoc.Array[0].Bulk)
	}
}

func TestCOMMAND_COUNT_ReturnsInteger11(t *testing.T) {
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

	_, _ = conn.Write([]byte("*2\r\n$7\r\nCOMMAND\r\n$5\r\nCOUNT\r\n"))
	reader := bufio.NewReader(conn)

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if line != ":11\r\n" {
		t.Fatalf("expected :11\\r\\n, got %q", line)
	}
}

func TestPING_WithOneArg_ReturnsThatArgAsBulk(t *testing.T) {
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

	// PING hello
	_, _ = conn.Write([]byte("*2\r\n$4\r\nPING\r\n$5\r\nhello\r\n"))
	reader := bufio.NewReader(conn)

	line1, _ := reader.ReadString('\n')
	if line1 != "$5\r\n" {
		t.Fatalf("expected $5\\r\\n, got %q", line1)
	}
	line2, _ := reader.ReadString('\n')
	if line2 != "hello\r\n" {
		t.Fatalf("expected hello\\r\\n, got %q", line2)
	}
}

func TestEXPIREAT_SetsExpiryAndTTLReflectsIt(t *testing.T) {
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

	// SET k v
	_, _ = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"))
	if line, _ := reader.ReadString('\n'); line != "+OK\r\n" {
		t.Fatalf("set: got %q", line)
	}

	// EXPIREAT k <future unix>
	future := time.Now().Add(30 * time.Second).Unix()
	tsStr := strconv.FormatInt(future, 10)
	cmd := "*3\r\n$8\r\nEXPIREAT\r\n$1\r\nk\r\n$" + strconv.Itoa(len(tsStr)) + "\r\n" + tsStr + "\r\n"
	_, _ = conn.Write([]byte(cmd))
	line, _ := reader.ReadString('\n')
	if line != ":1\r\n" {
		t.Fatalf("expected :1 for EXPIREAT, got %q", line)
	}

	// TTL k => non-negative
	_, _ = conn.Write([]byte("*2\r\n$3\r\nTTL\r\n$1\r\nk\r\n"))
	ttlLine, _ := reader.ReadString('\n')
	if !strings.HasPrefix(ttlLine, ":") {
		t.Fatalf("expected integer TTL, got %q", ttlLine)
	}
	if ttlLine == ":-1\r\n" || ttlLine == ":-2\r\n" {
		t.Fatalf("expected positive TTL, got %q", ttlLine)
	}
}
