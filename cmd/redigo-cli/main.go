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
	// Note: quoting is handled by the shell in one-shot mode.
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
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.EqualFold(line, "quit") || strings.EqualFold(line, "exit") {
			return
		}

		parts, err := parseArgs(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR %v\n", err)
			continue
		}
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

// parseArgs splits a single REPL line into args, supporting:
// - double quotes: "hello world"
// - escapes inside double quotes: \" \\ \n \t \r
// - single quotes: 'literal text' (no escapes)
// - backslash escapes outside quotes: \  -> space, \" -> ", etc.
func parseArgs(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	var args []string
	var cur strings.Builder

	const (
		stNormal = iota
		stInSingle
		stInDouble
		stEscape
	)
	state := stNormal
	prevState := stNormal

	flush := func() {
		if cur.Len() > 0 {
			args = append(args, cur.String())
			cur.Reset()
		}
	}

	for i := 0; i < len(line); i++ {
		c := line[i]

		switch state {
		case stNormal:
			switch c {
			case ' ', '\t':
				flush()
			case '\'':
				state = stInSingle
			case '"':
				state = stInDouble
			case '\\':
				prevState = stNormal
				state = stEscape
			default:
				cur.WriteByte(c)
			}

		case stInSingle:
			if c == '\'' {
				state = stNormal
			} else {
				cur.WriteByte(c)
			}

		case stInDouble:
			switch c {
			case '"':
				state = stNormal
			case '\\':
				prevState = stInDouble
				state = stEscape
			default:
				cur.WriteByte(c)
			}

		case stEscape:
			switch c {
			case 'n':
				cur.WriteByte('\n')
			case 'r':
				cur.WriteByte('\r')
			case 't':
				cur.WriteByte('\t')
			case '\\':
				cur.WriteByte('\\')
			case '"':
				cur.WriteByte('"')
			case '\'':
				cur.WriteByte('\'')
			case ' ':
				cur.WriteByte(' ')
			default:
				// forgiving: unknown escape becomes the literal char
				cur.WriteByte(c)
			}
			state = prevState
		}
	}

	if state == stInSingle || state == stInDouble {
		return nil, fmt.Errorf("unterminated quote")
	}
	if state == stEscape {
		return nil, fmt.Errorf("dangling escape at end of line")
	}

	flush()
	return args, nil
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
