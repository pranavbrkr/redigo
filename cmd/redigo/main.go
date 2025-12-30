package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
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
	log.Printf("client handler started for %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			payload := buf[:n]
			log.Printf("received %d bytes from %s", n, conn.RemoteAddr())
			log.Printf("raw: %q", string(buf[:n]))

			if bytes.Contains(payload, []byte("PING")) {
				_, _ = writer.WriteString("+PONG\r\n")
			} else {
				_, _ = writer.WriteString("-ERR unsupported (RESP parsing not implemented yet)\r\n")
			}

			if ferr := writer.Flush(); ferr != nil {
				log.Printf("flush error to %s: %v", conn.RemoteAddr(), ferr)
				return
			}
		}

		if err != nil {
			if err == io.EOF {
				log.Printf("clieent disconnected: %s", conn.RemoteAddr())
			} else {
				log.Printf("read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
	}
}
