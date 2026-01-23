package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
)

func main() {
	host := flag.String("h", "127.0.0.1", "server host")
	port := flag.Int("p", 6379, "server port")
	flag.Parse()

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR dial %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	// One-shot mode: redigo-cli PING
	if flag.NArg() > 0 {
		args := flag.Args()
		if err := sendCommand(w, args); err != nil {
			fmt.Fprintf(os.Stderr, "ERR write: %v\n", err)
			os.Exit(1)
		}
		v, err := resp.Decode(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR read: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(formatValue(v, 0))
		return
	}

	// REPL mode
	in := bufio.NewScanner(os.Stdin)
	fmt.Printf("Connected to %s\n", addr)

	for {
		fmt.Print("redigo> ")
		if !in.Scan() {
			// Ctrl+D / EOF
			fmt.Println()
			return
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "quit") || strings.EqualFold(line, "exit") {
			return
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		if err := sendCommand(w, parts); err != nil {
			fmt.Fprintf(os.Stderr, "ERR write: %v\n", err)
			continue
		}

		v, err := resp.Decode(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR read: %v\n", err)
			continue
		}

		fmt.Println(formatValue(v, 0))
	}
}

func sendCommand(w *bufio.Writer, parts []string) error {
	// Encode as RESP Array of Bulk Strings: [cmd, arg1, arg2, ...]
	if err := resp.WriteArrayHeader(w, len(parts)); err != nil {
		return err
	}
	for _, p := range parts {
		if err := resp.WriteBulkString(w, []byte(p)); err != nil {
			return err
		}
	}
	return w.Flush()
}

func formatValue(v resp.Value, indent int) string {
	switch v.Type {
	case resp.SimpleString:
		return v.Str
	case resp.Error:
		return "(error) " + v.Str
	case resp.Integer:
		return "(integer) " + strconv.FormatInt(v.Int, 10)
	case resp.BulkString:
		if v.Bulk == nil {
			return "(nil)"
		}
		return string(v.Bulk)
	case resp.Array:
		if v.Array == nil {
			return "(nil)"
		}
		if len(v.Array) == 0 {
			return "(empty array)"
		}
		var b strings.Builder
		for i, it := range v.Array {
			b.WriteString(fmt.Sprintf("%d) %s", i+1, formatValue(it, indent+1)))
			if i != len(v.Array)-1 {
				b.WriteByte('\n')
			}
		}
		return b.String()
	default:
		return "(unknown)"
	}
}
