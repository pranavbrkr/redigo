package main

import (
	"bufio"
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
			if err == io.EOF {
				log.Printf("clieent disconnected: %s", conn.RemoteAddr())
				return
			}
			log.Printf("decode error from %s: %v", conn.RemoteAddr(), err)
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
