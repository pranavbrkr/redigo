package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
	"github.com/pranavbrkr/redigo/internal/store"
)

type Server struct {
	ln    *net.TCPListener
	store *store.Store
}

func Start(addr string, st *store.Store) (*Server, string, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, "", err
	}

	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, "", err
	}
	s := &Server{ln: ln, store: st}

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
		go handleConn(conn, s.store)
	}
}

func handleConn(conn net.Conn, st *store.Store) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		v, err := resp.Decode(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || isConnReset(err) {
				return
			}
			_ = resp.WriteError(writer, "ERR protocol error")
			_ = writer.Flush()
			return
		}

		cmd, args, ok := decodeCommandParts(v)
		if !ok {
			_ = resp.WriteError(writer, "-ERR expected array of bulk strings\r\n")
			_ = writer.Flush()
			continue
		}

		switch cmd {
		case "PING":
			_ = resp.WriteSimpleString(writer, "PONG")

		case "ECHO":
			if len(args) < 1 {
				_ = resp.WriteError(writer, "ERR wrong number of argunments for 'echo' command")
				break
			}
			_ = resp.WriteBulkString(writer, []byte(args[0]))

		case "SET":
			if len(args) < 2 {
				_ = resp.WriteError(writer, "ERR wrong number of arguments for 'set' command")
				break
			}
			key := args[0]
			val := []byte(args[1])
			st.Set(key, val)
			_ = resp.WriteSimpleString(writer, "OK")

		case "GET":
			if len(args) < 1 {
				_ = resp.WriteError(writer, "ERR wrong number fo arguments for 'get' command")
				break
			}
			key := args[0]
			val, ok := st.Get(key)
			if !ok {
				_ = resp.WriteBulkString(writer, nil)
				break
			}
			_ = resp.WriteBulkString(writer, val)

		case "DEL":
			if len(args) < 1 {
				_ = resp.WriteError(writer, "ERR wrong number of arguments for 'del' command")
				break
			}
			var removed int64 = 0
			for _, key := range args {
				if st.Del(key) {
					removed++
				}
			}
			_ = resp.WriteInteger(writer, removed)

		case "EXISTS":
			if len(args) < 1 {
				_ = resp.WriteError(writer, "ERR wrong number of arguments for 'exists' command")
				break
			}

			var count int64 = 0
			for _, key := range args {
				if st.Exists(key) {
					count++
				}
			}
			_ = resp.WriteInteger(writer, count)
		case "COMMAND":
			_ = resp.WriteArrayHeader(writer, 0)

		case "INFO":
			info := []byte("# Server\r\nredigo:1\r\n")
			_ = resp.WriteBulkString(writer, info)

		default:
			_ = resp.WriteError(writer, "ERR unknown command")
		}

		if err := writer.Flush(); err != nil {
			return
		}
	}
}

func decodeCommandParts(v resp.Value) (string, []string, bool) {
	if v.Type != resp.Array || len(v.Array) == 0 {
		return "", nil, false
	}

	parts := make([]string, 0, len(v.Array))
	for _, item := range v.Array {
		if item.Type != resp.BulkString || item.Bulk == nil {
			return "", nil, false
		}
		parts = append(parts, string(item.Bulk))
	}

	cmd := strings.ToUpper(parts[0])
	args := parts[1:]
	return cmd, args, true

}

func isConnReset(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "connection reset")
}
