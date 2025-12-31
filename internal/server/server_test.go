package server

import (
	"bufio"
	"net"
	"testing"
	"time"
)

// Small timeout to prevent hanging forever
func TestPing(t *testing.T) {
	s, addr, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send RESP2 for: PING_
	_, err = conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if line != "+PONG\r\n" {
		t.Fatalf("expected +PONG\\r\\n, got %q", line)
	}

}
