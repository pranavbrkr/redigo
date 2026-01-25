package aof

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestRewriteInstallAppendsTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	st := store.New()
	st.Set("a", []byte("1"))

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open aof: %v", err)
	}
	defer aw.Close()

	// snapshot with a=1
	snap := st.Snapshot()

	tmp, err := aw.WriteRewriteTemp(snap)
	if err != nil {
		t.Fatalf("write temp: %v", err)
	}

	// tail: set b=2, expire a
	exp := time.Now().Add(10 * time.Second).Unix()
	tail := []Entry{
		{Cmd: "SET", Args: []string{"b", "2"}},
		{Cmd: "EXPIREAT", Args: []string{"a", strconv.FormatInt(exp, 10)}},
	}

	if err := aw.InstallRewrite(tmp, tail); err != nil {
		t.Fatalf("install rewrite: %v", err)
	}

	// Replay new AOF and validate
	st2 := store.New()
	err = Replay(path, func(cmd string, args []string) error {
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
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if v, ok := st2.Get("a"); !ok || string(v) != "1" {
		t.Fatalf("expected a=1, got ok=%v v=%q", ok, string(v))
	}
	if v, ok := st2.Get("b"); !ok || string(v) != "2" {
		t.Fatalf("expected b=2, got ok=%v v=%q", ok, string(v))
	}
	if ttl := st2.TTL("a"); ttl <= 0 {
		t.Fatalf("expected a ttl > 0, got %d", ttl)
	}
}
