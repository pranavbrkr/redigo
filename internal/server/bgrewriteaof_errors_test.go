package server

import (
	"bufio"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/protocol/resp"
	"github.com/pranavbrkr/redigo/internal/store"
)

func writeCommand(conn net.Conn, parts ...string) {
	w := bufio.NewWriter(conn)
	_ = resp.WriteArrayHeader(w, len(parts))
	for _, p := range parts {
		_ = resp.WriteBulkString(w, []byte(p))
	}
	_ = w.Flush()
}

func TestBGREWRITEAOF_WithoutAOF_ReturnsError(t *testing.T) {
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

	writeCommand(conn, "BGREWRITEAOF")
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected error, got %q", line)
	}
	if !strings.Contains(strings.ToLower(line), "aof rewrite not supported") {
		t.Fatalf("expected 'aof rewrite not supported', got %q", line)
	}
}

func TestBGREWRITEAOF_AlreadyInProgress_SecondReturnsError(t *testing.T) {
	dir := t.TempDir()
	aofPath := filepath.Join(dir, "appendonly.aof")

	aw, err := aof.Open(aofPath)
	if err != nil {
		t.Fatalf("open aof: %v", err)
	}
	defer aw.Close()

	st := store.New()
	s, addr, err := Start("127.0.0.1:0", st, aw, aof.FsyncNever)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	// Preload so first rewrite has work and is more likely still running when we send second
	preloadConn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial preload: %v", err)
	}
	preloadR := bufio.NewReader(preloadConn)
	for i := 0; i < 500; i++ {
		// Vary key so snapshot has many entries and rewrite takes longer
		_, _ = preloadConn.Write([]byte("*3\r\n$3\r\nSET\r\n$7\r\npre:k\r\n$1\r\nx\r\n"))
		_, _ = preloadR.ReadString('\n')
	}
	// Overwrite one key 500 times so AOF has many entries and rewrite has work
	for i := 0; i < 500; i++ {
		_, _ = preloadConn.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\na\r\n$3\r\nval\r\n"))
		_, _ = preloadR.ReadString('\n')
	}
	_ = preloadConn.Close()

	conn1, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}
	defer conn2.Close()

	r1 := bufio.NewReader(conn1)
	r2 := bufio.NewReader(conn2)

	// First BGREWRITEAOF
	writeCommand(conn1, "BGREWRITEAOF")
	line1, _ := r1.ReadString('\n')
	if line1 != "+OK\r\n" {
		t.Fatalf("first BGREWRITEAOF expected +OK, got %q", line1)
	}

	// Second BGREWRITEAOF immediately (rewrite may still be running)
	writeCommand(conn2, "BGREWRITEAOF")
	line2, _ := r2.ReadString('\n')
	if !strings.HasPrefix(line2, "-") {
		t.Fatalf("second BGREWRITEAOF expected error, got %q", line2)
	}
	if !strings.Contains(strings.ToLower(line2), "already in progress") {
		t.Fatalf("expected 'already in progress', got %q", line2)
	}
}
