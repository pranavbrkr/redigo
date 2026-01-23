package aof

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestReplayAppliesExpireAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open aof: %v", err)
	}
	defer aw.Close()

	// SET k v
	if err := aw.Append("SET", []string{"k", "v"}); err != nil {
		t.Fatalf("append set: %v", err)
	}

	// EXPIREAT k (now-1) => should delete on replay
	past := time.Now().Add(-1 * time.Second).Unix()
	if err := aw.Append("EXPIREAT", []string{"k", strconv.FormatInt(past, 10)}); err != nil {
		t.Fatalf("append expireat: %v", err)
	}

	_ = aw.Sync()

	st := store.New()

	err = Replay(path, func(cmd string, args []string) error {
		switch cmd {
		case "SET":
			if len(args) == 2 {
				st.Set(args[0], []byte(args[1]))
			}
		case "EXPIREAT":
			if len(args) == 2 {
				unixSec, e := strconv.ParseInt(args[1], 10, 64)
				if e == nil {
					st.ExpireAt(args[0], unixSec)
				}
			}
		case "EXPIRE":
			// backwards compat (optional)
			if len(args) == 2 {
				sec, e := strconv.ParseInt(args[1], 10, 64)
				if e == nil {
					st.Expire(args[0], sec)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if st.Exists("k") {
		t.Fatalf("expected key to be absent after replaying past EXPIREAT")
	}
}
