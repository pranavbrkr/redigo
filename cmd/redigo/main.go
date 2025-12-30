package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
)

func main() {
	port := flag.Int("port", 6379, "TCP port to listen on")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	defer ln.Close()

	log.Printf("redigo listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}

		log.Printf("accepted connection from %s", conn.RemoteAddr())
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
				log.Printf("client disconnected: %s", conn.RemoteAddr())
				return
			}
			log.Printf("decode error from %s: %v", conn.RemoteAddr(), err)
			if _, werr := writer.WriteString("-ERR protocol error\r\n"); werr != nil {
				log.Printf("write error to %s: %v", conn.RemoteAddr(), werr)
				return
			}
			if ferr := writer.Flush(); ferr != nil {
				log.Printf("flush error to %s: %v", conn.RemoteAddr(), ferr)
			}
			return
		}

		log.Printf("decoded value type=%v from %s", v.Type, conn.RemoteAddr())

		cmd, ok := decodeCommand(v)
		log.Printf("command=%s from %s", cmd, conn.RemoteAddr())
		if !ok {
			if _, werr := writer.WriteString("-ERR expected arry of bulk strings\r\n"); werr != nil {
				log.Printf("write error to %s: %v", conn.RemoteAddr(), werr)
				return
			}
			if ferr := writer.Flush(); ferr != nil {
				log.Printf("flush error to %s: %v", conn.RemoteAddr(), ferr)
				return
			}
			continue
		}

		switch cmd {
		case "PING":
			if _, werr := writer.WriteString("+PONG\r\n"); werr != nil {
				log.Printf("write error to %s: %v", conn.RemoteAddr(), werr)
				return
			}
		default:
			if _, werr := writer.WriteString("-ERR unknown command\r\n"); werr != nil {
				log.Printf("write error to %s: %v", conn.RemoteAddr(), werr)
				return
			}
		}

		if err := writer.Flush(); err != nil {
			log.Printf("flush error to %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

// Extracts the command name from a RESP array of Bulk strings
func decodeCommand(v resp.Value) (string, bool) {
	if v.Type != resp.Array || len(v.Array) == 0 {
		return "", false
	}

	first := v.Array[0]
	if first.Type != resp.BulkString || first.Bulk == nil {
		return "", false
	}

	cmd := strings.ToUpper(string(first.Bulk))
	return cmd, true
}

// Windows aptch
// Treat as normal disconnect for now
func isConnReset(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "connection reset")
}
