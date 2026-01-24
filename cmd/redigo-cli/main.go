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
	"unicode"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
)

func main() {
	host := flag.String("h", "127.0.0.1", "server host")
	port := flag.Int("p", 6379, "server port")
	raw := flag.Bool("raw", false, "Raw output (no quotes/prefixes); useful for scripting")
	timeout := flag.Duration("timeout", 3*time.Second, "Dial/read timeout (e.g. 3s, 500ms)")
	flag.Parse()

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))

	conn, err := net.DialTimeout("tcp", addr, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR dial %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Time{}) // we will set read deadlines per operation

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	// 1) One-shot mode: redigo-cli PING
	if flag.NArg() > 0 {
		args := flag.Args()
		if err := sendAndPrintOne(conn, r, w, args, *timeout, formatOpts{raw: *raw}); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	// 2) Pipe mode: stdin is not a terminal and no args
	if !stdinIsTerminal() {
		if err := runPipeMode(conn, r, w, *timeout, formatOpts{raw: *raw}); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	// 3) REPL mode
	runREPL(addr, conn, r, w, *timeout, formatOpts{raw: *raw})

}

// ---------- modes ----------

func runREPL(addr string, conn net.Conn, r *bufio.Reader, w *bufio.Writer, timeout time.Duration, opts formatOpts) {
	in := bufio.NewScanner(os.Stdin)
	fmt.Printf("Connected to %s\n", addr)

	for {
		fmt.Print("redigo> ")
		if !in.Scan() {
			fmt.Println()
			return
		}

		line := strings.TrimSpace(in.Text())
		if line == "" || strings.HasPrefix(line, "#") {
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

		if err := sendAndPrintOne(conn, r, w, parts, timeout, opts); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}
	}
}

func runPipeMode(conn net.Conn, r *bufio.Reader, w *bufio.Writer, timeout time.Duration, opts formatOpts) error {
	sc := bufio.NewScanner(os.Stdin)
	// allow longer lines (default is 64K)
	buf := make([]byte, 0, 256*1024)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts, err := parseArgs(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERR", err)
			continue
		}
		if len(parts) == 0 {
			continue
		}
		if err := sendAndPrintOne(conn, r, w, parts, timeout, opts); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("ERR reading stdin: %v", err)
	}
	return nil
}

func sendAndPrintOne(conn net.Conn, r *bufio.Reader, w *bufio.Writer, parts []string, timeout time.Duration, opts formatOpts) error {
	// Write deadline
	if timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	if err := sendCommand(w, parts); err != nil {
		_ = conn.SetWriteDeadline(time.Time{})
		return fmt.Errorf("ERR write: %v", err)
	}
	_ = conn.SetWriteDeadline(time.Time{})

	// Read deadline
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	v, err := resp.Decode(r)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("ERR read: %v", err)
	}

	out := formatValue(v, opts)
	fmt.Print(out)
	if !strings.HasSuffix(out, "\n") {
		fmt.Println()
	}
	return nil
}

// ---------- helpers ----------

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		// assume terminal if unsure
		return true
	}
	// On Windows this is a decent heuristic:
	// if stdin is a char device, it’s interactive; otherwise it’s pipe/file.
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func sendCommand(w *bufio.Writer, parts []string) error {
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

// ---------- formatting (same as 7J.3) ----------

type formatOpts struct {
	raw bool
}

func formatValue(v resp.Value, opts formatOpts) string {
	if opts.raw {
		return formatRaw(v)
	}
	return formatPretty(v)
}

func formatPretty(v resp.Value) string {
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
		return quoteRedisString(string(v.Bulk))
	case resp.Array:
		if v.Array == nil {
			return "(nil)"
		}
		if len(v.Array) == 0 {
			return "(empty array)"
		}
		var b strings.Builder
		for i, it := range v.Array {
			b.WriteString(fmt.Sprintf("%d) %s", i+1, formatPretty(it)))
			if i != len(v.Array)-1 {
				b.WriteByte('\n')
			}
		}
		return b.String()
	default:
		return "(unknown)"
	}
}

func formatRaw(v resp.Value) string {
	switch v.Type {
	case resp.SimpleString:
		return v.Str
	case resp.Error:
		return v.Str
	case resp.Integer:
		return strconv.FormatInt(v.Int, 10)
	case resp.BulkString:
		if v.Bulk == nil {
			return ""
		}
		return string(v.Bulk)
	case resp.Array:
		if len(v.Array) == 0 {
			return ""
		}
		var b strings.Builder
		for i, it := range v.Array {
			b.WriteString(formatRaw(it))
			if i != len(v.Array)-1 {
				b.WriteByte('\n')
			}
		}
		return b.String()
	default:
		return ""
	}
}

func quoteRedisString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if unicode.IsControl(r) {
				b.WriteString(fmt.Sprintf(`\x%02x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ---------- parseArgs (same as your 7J.3) ----------

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
