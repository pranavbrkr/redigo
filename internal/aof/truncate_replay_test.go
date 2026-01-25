// internal/aof/truncate_replay_test.go

package aof

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pranavbrkr/redigo/internal/store"
)

func TestReplayIgnoresTruncatedTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open aof: %v", err)
	}
	if err := aw.Append("SET", []string{"k", "v"}); err != nil {
		t.Fatalf("append set: %v", err)
	}
	_ = aw.Sync()
	_ = aw.Close()

	// Append a truncated command tail (simulate crash mid-write)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	_, _ = f.Write([]byte("*3\r\n$3\r\nSET\r\n$1\r\nx\r\n$")) // truncated bulk len
	_ = f.Close()

	st := store.New()
	if err := Replay(path, func(cmd string, args []string) error {
		if cmd == "SET" && len(args) == 2 {
			st.Set(args[0], []byte(args[1]))
		}
		return nil
	}); err != nil {
		t.Fatalf("expected replay to succeed, got: %v", err)
	}

	if got, ok := st.Get("k"); !ok || string(got) != "v" {
		t.Fatalf("expected k=v to be applied, got ok=%v val=%q", ok, string(got))
	}
}
