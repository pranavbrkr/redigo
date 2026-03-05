package store

import (
	"testing"
)

func TestDel_MissingKey_ReturnsFalse(t *testing.T) {
	s := New()
	if s.Del("missing") {
		t.Fatal("expected Del(missing) to return false")
	}
}

func TestDel_ExistingKey_ReturnsTrueAndRemoves(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))
	if !s.Del("k") {
		t.Fatal("expected Del(k) to return true")
	}
	if _, ok := s.Get("k"); ok {
		t.Fatal("expected key k to be removed")
	}
}

func TestDel_SecondDelSameKey_ReturnsFalse(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))
	s.Del("k")
	if s.Del("k") {
		t.Fatal("expected second Del(k) to return false")
	}
}

func TestExists_MultipleKeys_ReturnsCorrectCount(t *testing.T) {
	s := New()
	s.Set("a", []byte("1"))
	s.Set("b", []byte("2"))

	if !s.Exists("a") {
		t.Fatal("expected Exists(a) true")
	}
	if !s.Exists("b") {
		t.Fatal("expected Exists(b) true")
	}
	if s.Exists("c") {
		t.Fatal("expected Exists(c) false")
	}
	// Two existing
	if n := countExists(s, "a", "b"); n != 2 {
		t.Fatalf("expected 2 existing, got %d", n)
	}
	// One existing, one missing
	if n := countExists(s, "a", "c"); n != 1 {
		t.Fatalf("expected 1 existing, got %d", n)
	}
}

func countExists(s *Store, keys ...string) int {
	n := 0
	for _, k := range keys {
		if s.Exists(k) {
			n++
		}
	}
	return n
}
