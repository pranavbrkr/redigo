package aof

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFileAOF_AppendAfterClose_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := aw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	err = aw.Append("SET", []string{"k", "v"})
	if err == nil {
		t.Fatal("expected error from Append after Close")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected error to mention closed, got %v", err)
	}
}

func TestFileAOF_SyncAfterClose_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := aw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Sync after close is documented as no-op (returns nil in Close; Sync on closed returns nil)
	err = aw.Sync()
	if err != nil {
		t.Fatalf("Sync after Close: expected nil (no-op), got %v", err)
	}
}

func TestFileAOF_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	aw, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := aw.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := aw.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
