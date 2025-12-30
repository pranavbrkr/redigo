package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
)

type Server struct {
	ln *net.TCPListener
}

func Start(addr string) (*Server, string, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, "", err
	}

	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, "", err
	}
	s := &Server{ln: ln}

	go s.acceptLoop()

	return s, ln.Addr().String(), nil
}

func (s *Server) Close() error {
	if s.ln == nil {
		return nil
	}

	return s.ln.Close()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		v, err := resp.Decode(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || isConnReset(err) {
				return
			}
			_, _ = writer.WriteString("-ERR protocol error\r\n")
			_ = writer.Flush()
			return
		}

		cmd, ok := decodeCommand(v)
		if !ok {
			_, _ = writer.WriteString("-ERR expected array of bulk strings\r\n")
			_ = writer.Flush()
			continue
		}

		switch cmd {
		case "PING":
			_, _ = writer.WriteString("+PONG\r\n")
		default:
			_, _ = writer.WriteString("-ERR unknown command\r\n")
		}

		if err := writer.Flush(); err != nil {
			return
		}
	}
}

func decodeCommand(v resp.Value) (string, bool) {
	if v.Type != resp.Array || len(v.Array) == 0 {
		return "", false
	}

	first := v.Array[0]
	if first.Type != resp.BulkString || first.Bulk == nil {
		return "", false
	}

	return strings.ToUpper(string(first.Bulk)), true
}

func isConnReset(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "connection reset")
}
