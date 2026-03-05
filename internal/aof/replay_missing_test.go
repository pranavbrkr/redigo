package aof

import (
	"path/filepath"
	"testing"
)

func TestReplay_MissingFile_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.aof")

	applyCount := 0
	err := Replay(path, func(cmd string, args []string) error {
		applyCount++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if applyCount != 0 {
		t.Fatalf("expected apply not to be called, got %d calls", applyCount)
	}
}
