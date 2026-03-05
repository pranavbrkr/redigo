package store

import (
	"testing"
	"time"
)

func TestSnapshot_EmptyStore_ReturnsEmptySlice(t *testing.T) {
	s := New()
	snap := s.Snapshot()
	if snap == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(snap) != 0 {
		t.Fatalf("expected empty snapshot, got %d entries", len(snap))
	}
}

func TestSnapshot_ContainsNonExpiredKeysWithValuesAndExpiry(t *testing.T) {
	s := New()
	s.Set("a", []byte("1"))
	s.Set("b", []byte("2"))
	future := time.Now().Add(10 * time.Second).Unix()
	s.ExpireAt("b", future)

	snap := s.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(snap))
	}

	var gotA, gotB bool
	for _, e := range snap {
		if e.Key == "a" {
			gotA = true
			if string(e.Value) != "1" {
				t.Fatalf("key a: expected value 1, got %q", e.Value)
			}
			if e.ExpiresAt != nil {
				t.Fatalf("key a: expected no expiry, got %v", e.ExpiresAt)
			}
		}
		if e.Key == "b" {
			gotB = true
			if string(e.Value) != "2" {
				t.Fatalf("key b: expected value 2, got %q", e.Value)
			}
			if e.ExpiresAt == nil || *e.ExpiresAt != future {
				t.Fatalf("key b: expected ExpiresAt %d, got %v", future, e.ExpiresAt)
			}
		}
	}
	if !gotA || !gotB {
		t.Fatalf("missing keys in snapshot: a=%v b=%v", gotA, gotB)
	}
}

func TestSnapshot_PurgesExpiredKeys(t *testing.T) {
	s := New()
	s.Set("a", []byte("1"))
	past := time.Now().Add(-1 * time.Second).Unix()
	s.ExpireAt("a", past)

	snap := s.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("expected expired key purged from snapshot, got %d entries", len(snap))
	}
	// Key should be removed from store as well
	if _, ok := s.Get("a"); ok {
		t.Fatal("expected key a to be purged from store after snapshot")
	}
}
