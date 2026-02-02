package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/protocol/resp"
	"github.com/pranavbrkr/redigo/internal/store"
)

// Regression test: ensure BGREWRITEAOF never loses concurrent SET writes.
// The old bug window was: tail captured before AOF appends were blocked,
// allowing writes to land in the old AOF and never make it into the rewritten file.
func TestBGREWRITEAOF_DoesNotLoseConcurrentWrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	aofPath := filepath.Join(dir, "appendonly.aof")

	st := store.New()

	aw, err := aof.Open(aofPath)
	if err != nil {
		t.Fatalf("open aof: %v", err)
	}
	defer aw.Close()

	s, addr, err := Start("127.0.0.1:0", st, aw, aof.FsyncNever)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	// Conn 1: writer
	wConn, wR, wW := mustDial(t, addr)
	defer wConn.Close()

	// Conn 2: admin (BGREWRITEAOF)
	aConn, aR, aW := mustDial(t, addr)
	defer aConn.Close()

	// Preload a bit of state so rewrite has something non-trivial to snapshot.
	// (This also makes the rewrite more likely to overlap with concurrent writes.)
	for i := 0; i < 2000; i++ {
		if err := sendCmd(wW, "SET", "pre:"+strconv.Itoa(i), "x"); err != nil {
			t.Fatalf("preload write: %v", err)
		}
		if err := expectSimpleOK(wR); err != nil {
			t.Fatalf("preload resp: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	var wrote int64
	var wg sync.WaitGroup

	// Writer goroutine: keep writing new keys rapidly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			key := "k:" + strconv.Itoa(i)
			val := "v:" + strconv.Itoa(i)

			if err := sendCmd(wW, "SET", key, val); err != nil {
				return
			}
			if err := expectSimpleOK(wR); err != nil {
				return
			}

			atomic.StoreInt64(&wrote, int64(i+1))
			i++
		}
	}()

	// Rewriter loop: trigger BGREWRITEAOF repeatedly while writes are happening.
	// Repeating increases the chance of hitting the old race window deterministically.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := sendCmd(aW, "BGREWRITEAOF"); err != nil {
				return
			}
			// Server replies +OK immediately (it runs rewrite async)
			_ = expectSimpleOK(aR)

			// Small jitter so we don’t just spam in a tight loop
			time.Sleep(20 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Stop server so buffers are flushed.
	_ = s.Close()
	_ = aw.Close()

	total := int(atomic.LoadInt64(&wrote))
	if total == 0 {
		t.Fatalf("expected to write some keys")
	}

	// Replay AOF into a fresh store (simulates restart recovery).
	st2 := store.New()
	if err := aof.Replay(aofPath, func(cmd string, args []string) error {
		switch cmd {
		case "SET":
			if len(args) == 2 {
				st2.Set(args[0], []byte(args[1]))
			}
		case "DEL":
			for _, k := range args {
				st2.Del(k)
			}
		case "EXPIREAT":
			if len(args) == 2 {
				ts, e := strconv.ParseInt(args[1], 10, 64)
				if e == nil {
					st2.ExpireAt(args[0], ts)
				}
			}
		case "EXPIRE":
			// backwards compat
			if len(args) == 2 {
				sec, e := strconv.ParseInt(args[1], 10, 64)
				if e == nil {
					st2.Expire(args[0], sec)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("replay: %v", err)
	}

	// Validate: every concurrently written key must exist with correct value after replay.
	for i := 0; i < total; i++ {
		k := "k:" + strconv.Itoa(i)
		want := "v:" + strconv.Itoa(i)

		got, ok := st2.Get(k)
		if !ok {
			t.Fatalf("missing key after replay (possible lost write): %s (total=%d)", k, total)
		}
		if string(got) != want {
			t.Fatalf("wrong value after replay for %s: got=%q want=%q", k, string(got), want)
		}
	}
}

// ---- test helpers ----

func mustDial(t *testing.T, addr string) (net.Conn, *bufio.Reader, *bufio.Writer) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn, bufio.NewReader(conn), bufio.NewWriter(conn)
}

func sendCmd(w *bufio.Writer, parts ...string) error {
	if len(parts) == 0 {
		return fmt.Errorf("no command parts")
	}
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

func expectSimpleOK(r *bufio.Reader) error {
	v, err := resp.Decode(r)
	if err != nil {
		return err
	}
	if v.Type != resp.SimpleString || v.Str != "OK" {
		return fmt.Errorf("expected +OK, got type=%v str=%q", v.Type, v.Str)
	}
	return nil
}
