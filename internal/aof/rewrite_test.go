// internal/aof/rewrite_test.go

package aof

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestRewriteCompactsAndReplaysState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	// Create state
	st := store.New()
	st.Set("a", []byte("1"))
	st.Set("b", []byte("2"))
	exp := time.Now().Add(10 * time.Second).Unix()
	st.ExpireAt("b", exp)

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open aof: %v", err)
	}
	defer aw.Close()

	// Pretend some history exists
	_ = aw.Append("SET", []string{"a", "old"})
	_ = aw.Append("DEL", []string{"a"})
	_ = aw.Append("SET", []string{"a", "1"})
	_ = aw.Append("SET", []string{"b", "2"})
	_ = aw.Append("EXPIREAT", []string{"b", strconv.FormatInt(exp, 10)})
	_ = aw.Sync()

	// Rewrite from snapshot
	snap := st.Snapshot()
	if err := aw.Rewrite(snap); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// Replay rewritten AOF into a fresh store
	st2 := store.New()
	if err := Replay(path, func(cmd string, args []string) error {
		switch cmd {
		case "SET":
			if len(args) == 2 {
				st2.Set(args[0], []byte(args[1]))
			}
		case "EXPIREAT":
			if len(args) == 2 {
				ts, _ := strconv.ParseInt(args[1], 10, 64)
				st2.ExpireAt(args[0], ts)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("replay rewritten: %v", err)
	}

	if v, ok := st2.Get("a"); !ok || string(v) != "1" {
		t.Fatalf("expected a=1 after replay, got ok=%v v=%q", ok, string(v))
	}

	ttl := st2.TTL("b")
	if ttl <= 0 {
		t.Fatalf("expected b to have positive ttl after replay, got ttl=%d", ttl)
	}
}
