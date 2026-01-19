package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
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
		s.stopFsync = startFsyncLoop(s.aof, 1*time.Second)
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
		go handleConn(conn, s.store, s.aof, s.fsyncPolicy)
	}
}

func handleConn(conn net.Conn, st *store.Store, aw aof.Writer, policy aof.FsyncPolicy) {
	if aw == nil {
		aw = aof.NewNoop()
	}

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
			val := args[1]

			// AOF first, then apply
			if !appendOrErr(writer, aw, policy, "SET", []string{key, val}) {
				return
			}

			st.Set(key, []byte(val))
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

			// If nothing would be removed, Redis still returns 0 and no state change
			if removed == 0 {
				_ = resp.WriteInteger(writer, 0)
				break
			}

			if !appendOrErr(writer, aw, policy, "DEL", args) {
				return
			}

			// Apply deletes
			for _, key := range args {
				st.Del(key)
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

		case "EXPIRE":
			if len(args) != 2 {
				_ = resp.WriteError(writer, "ERR wrong number of arguments for 'expire' command")
				break
			}
			seconds, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				_ = resp.WriteError(writer, "ERR value is not an integer or out of range")
				break
			}

			// Only append if it will actually set  expiry
			if !st.Exists(args[0]) {
				_ = resp.WriteInteger(writer, 0)
				break
			}

			if !appendOrErr(writer, aw, policy, "EXPIRE", []string{args[0], args[1]}) {
				return
			}

			ok := st.Expire(args[0], seconds)
			if ok {
				_ = resp.WriteInteger(writer, 1)
			} else {
				_ = resp.WriteInteger(writer, 0)
			}

		case "TTL":
			if len(args) != 1 {
				_ = resp.WriteError(writer, "ERR wrong number of arguments for 'ttl' command")
				break
			}
			ttl := st.TTL(args[0])
			_ = resp.WriteInteger(writer, ttl)

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

func appendOrErr(writer *bufio.Writer, aw aof.Writer, policy aof.FsyncPolicy, cmd string, args []string) bool {
	if err := aw.Append(cmd, args); err != nil {
		_ = resp.WriteError(writer, "ERR aof write failed")
		_ = writer.Flush()
		return false
	}
	if policy == aof.FsyncAlways {
		if err := aw.Sync(); err != nil {
			_ = resp.WriteError(writer, "ERR aof sync failed")
			_ = writer.Flush()
			return false
		}
	}
	return true
}

func startFsyncLoop(aw aof.Writer, interval time.Duration) func() {
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
				_ = aw.Sync()
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
