package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/protocol/resp"
	"github.com/pranavbrkr/redigo/internal/store"
)

type Server struct {
	ln          *net.TCPListener
	store       *store.Store
	stopReaper  func()
	aof         aof.Writer
	fsyncPolicy aof.FsyncPolicy
	stopFsync   func()
	aofMu       sync.Mutex
}

func Start(addr string, st *store.Store, aw aof.Writer, fsyncPolicy aof.FsyncPolicy) (*Server, string, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, "", err
	}
	if aw == nil {
		aw = aof.NewNoop()
	}

	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, "", err
	}
	s := &Server{ln: ln, store: st, aof: aw, fsyncPolicy: fsyncPolicy}
	s.stopReaper = st.StartReaper(500 * time.Millisecond)

	if s.fsyncPolicy == aof.FsyncEverySecond {
		s.stopFsync = startFsyncLoop(s, 1*time.Second)
	}

	go s.acceptLoop()

	return s, ln.Addr().String(), nil
}

func (s *Server) Close() error {
	if s.ln == nil {
		return nil
	}

	if s.stopReaper != nil {
		s.stopReaper()
	}

	if s.stopFsync != nil {
		s.stopFsync()
	}
	_ = s.aof.Close()

	return s.ln.Close()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	st := s.store

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
			_ = resp.WriteError(writer, "ERR expected array of bulk strings")
			_ = writer.Flush()
			continue
		}

		switch cmd {
		case "PING":
			if len(args) == 0 {
				_ = resp.WriteSimpleString(writer, "PONG")
				break
			}
			if len(args) == 1 {
				_ = resp.WriteBulkString(writer, []byte(args[0]))
				break
			}
			writeWrongArgs(writer, "PING")

		case "ECHO":
			if len(args) != 1 {
				writeWrongArgs(writer, "ECHO")
				break
			}
			_ = resp.WriteBulkString(writer, []byte(args[0]))

		case "SET":
			if len(args) != 2 {
				writeWrongArgs(writer, "SET")
				break
			}
			key := args[0]
			val := args[1]

			// AOF first, then apply
			if err := s.appendAOF("SET", []string{key, val}); err != nil {
				writeAOFError(writer, "ERR aof write failed")
				return
			}

			st.Set(key, []byte(val))
			_ = resp.WriteSimpleString(writer, "OK")

		case "GET":
			if len(args) != 1 {
				writeWrongArgs(writer, "GET")
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
				writeWrongArgs(writer, "DEL")
				break
			}

			// Decide what will actually be deleted (EXISTS purges expired keys too)
			toDelete := make([]string, 0, len(args))
			for _, key := range args {
				if st.Exists(key) {
					toDelete = append(toDelete, key)
				}
			}

			if len(toDelete) == 0 {
				_ = resp.WriteInteger(writer, 0)
				break
			}

			// AOF first (durability), then apply
			if err := s.appendAOF("DEL", toDelete); err != nil {
				_ = resp.WriteError(writer, "ERR aof write failed")
				_ = writer.Flush()
				return
			}

			var removed int64
			for _, key := range toDelete {
				if st.Del(key) {
					removed++
				}
			}

			_ = resp.WriteInteger(writer, removed)

		case "EXISTS":
			if len(args) < 1 {
				writeWrongArgs(writer, "EXISTS")
				break
			}

			var count int64 = 0
			for _, key := range args {
				if st.Exists(key) {
					count++
				}
			}
			_ = resp.WriteInteger(writer, count)

		case "EXPIRE":
			if len(args) != 2 {
				writeWrongArgs(writer, "EXPIRE")
				break
			}

			seconds, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				_ = resp.WriteError(writer, "ERR value is not an integer or out of range")
				break
			}

			// Apply first (decides if state changes). Store will purge expired keys too.
			ok := st.Expire(args[0], seconds)
			if !ok {
				_ = resp.WriteInteger(writer, 0)
				break
			}

			// Now log to AOF (state change happened)
			if err := s.appendAOF("EXPIRE", []string{args[0], args[1]}); err != nil {
				_ = resp.WriteError(writer, "ERR aof write failed")
				_ = writer.Flush()
				return
			}

			_ = resp.WriteInteger(writer, 1)

		case "TTL":
			if len(args) != 1 {
				writeWrongArgs(writer, "TTL")
				break
			}
			ttl := st.TTL(args[0])
			_ = resp.WriteInteger(writer, ttl)

		case "COMMAND":
			if len(args) == 0 {
				// List supported commands
				_ = resp.WriteArrayHeader(writer, 9)

				writeCommandDoc(writer, "PING", -1, []string{"fast"})
				writeCommandDoc(writer, "ECHO", 2, []string{"fast"})
				writeCommandDoc(writer, "SET", 3, []string{"write"})
				writeCommandDoc(writer, "GET", 2, []string{"readonly", "fast"})
				writeCommandDoc(writer, "DEL", -2, []string{"write"})
				writeCommandDoc(writer, "EXISTS", -2, []string{"readonly", "fast"})
				writeCommandDoc(writer, "EXPIRE", 3, []string{"write", "fast"})
				writeCommandDoc(writer, "TTL", 2, []string{"readonly", "fast"})
				writeCommandDoc(writer, "INFO", -1, []string{"readonly"})

				break
			}

			if len(args) == 1 && strings.ToUpper(args[0]) == "COUNT" {
				_ = resp.WriteInteger(writer, 9)
				break
			}

			writeWrongArgs(writer, "COMMAND")

		case "INFO":
			if len(args) != 0 {
				writeWrongArgs(writer, "INFO")
				break
			}
			info := []byte(
				"# Server\r\n" +
					"redis_version:0.0.1\r\n" +
					"redigo:1\r\n" +
					"tcp_port:6379\r\n",
			)
			_ = resp.WriteBulkString(writer, info)

		default:
			_ = resp.WriteError(writer, "ERR unknown command '"+strings.ToLower(cmd)+"'")
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

func startFsyncLoop(s *Server, interval time.Duration) func() {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	done := make(chan struct{})

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				s.syncAOF()
			case <-done:
				return
			}
		}
	}()

	return func() { close(done) }
}

func (s *Server) appendAOF(cmd string, args []string) error {
	if s.aof == nil {
		return nil
	}

	s.aofMu.Lock()
	defer s.aofMu.Unlock()

	// Append the logical operation
	if err := s.aof.Append(cmd, args); err != nil {
		return err
	}

	// If policy is always, force durability right now
	if s.fsyncPolicy == aof.FsyncAlways {
		if err := s.aof.Sync(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) syncAOF() {
	if s.aof == nil {
		return
	}

	s.aofMu.Lock()
	defer s.aofMu.Unlock()

	_ = s.aof.Sync()
}

func writeWrongArgs(w *bufio.Writer, cmd string) {
	_ = resp.WriteError(w, "ERR wrong number of arguments for '"+strings.ToLower(cmd)+"' command")
}

func writeAOFError(w *bufio.Writer, msg string) {
	_ = resp.WriteError(w, msg)
	_ = w.Flush()
}

func writeCommandDoc(w *bufio.Writer, name string, arity int64, flags []string) {
	// Format loosely matches Redis COMMAND output:
	// [name, arity, [flags...]]
	_ = resp.WriteArrayHeader(w, 3)
	_ = resp.WriteBulkString(w, []byte(strings.ToLower(name)))
	_ = resp.WriteInteger(w, arity)

	_ = resp.WriteArrayHeader(w, int(len(flags)))
	for _, f := range flags {
		_ = resp.WriteBulkString(w, []byte(f))
	}
}
