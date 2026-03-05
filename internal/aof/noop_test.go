package aof

import (
	"testing"
)

func TestNoop_Append_ReturnsNil(t *testing.T) {
	n := NewNoop()
	if err := n.Append("SET", []string{"k", "v"}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNoop_Sync_ReturnsNil(t *testing.T) {
	n := NewNoop()
	if err := n.Sync(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNoop_Close_ReturnsNil(t *testing.T) {
	n := NewNoop()
	if err := n.Close(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
