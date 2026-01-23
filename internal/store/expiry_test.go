package store

import (
	"testing"
	"time"
)

func TestTTLReturnsMinus2ForMissingKey(t *testing.T) {
	s := New()
	if got := s.TTL("missing"); got != -2 {
		t.Fatalf("expected -2 for missing key, got %d", got)
	}
}

func TestTTLReturnsMinus1ForKeyWithNoExpiry(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))
	if got := s.TTL("k"); got != -1 {
		t.Fatalf("expected -1 for key with no expiry, got %d", got)
	}
}

func TestSetClearsExistingExpiry(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))
	if ok := s.Expire("k", 10); !ok {
		t.Fatalf("expected expire to succeed")
	}

	// overwrite should clear expiry
	s.Set("k", []byte("v2"))

	if got := s.TTL("k"); got != -1 {
		t.Fatalf("expected TTL -1 after SET clears expiry, got %d", got)
	}
}

func TestExpireNonPositiveDeletesKeyAndReturnsTrue(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))

	if ok := s.Expire("k", 0); !ok {
		t.Fatalf("expected expire(0) to succeed")
	}

	if _, ok := s.Get("k"); ok {
		t.Fatalf("expected key to be deleted immediately on expire(0)")
	}
}

func TestExpireAtMissingKeyReturnsFalse(t *testing.T) {
	s := New()
	if ok := s.ExpireAt("missing", time.Now().Add(10*time.Second).Unix()); ok {
		t.Fatalf("expected ExpireAt on missing key to return false")
	}
}

func TestExpireAtPastDeletesKeyImmediately(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))

	past := time.Now().Add(-2 * time.Second).Unix()
	if ok := s.ExpireAt("k", past); !ok {
		t.Fatalf("expected ExpireAt(past) to return true for existing key")
	}

	if _, ok := s.Get("k"); ok {
		t.Fatalf("expected key to be deleted immediately for past ExpireAt")
	}
}

func TestExpireAtFutureExpiresEventually(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"))

	future := time.Now().Add(1 * time.Second).Unix()
	if ok := s.ExpireAt("k", future); !ok {
		t.Fatalf("expected ExpireAt(future) to succeed")
	}

	// immediately it should exist
	if _, ok := s.Get("k"); !ok {
		t.Fatalf("expected key to exist right after setting future ExpireAt")
	}

	// wait past expiry
	time.Sleep(1200 * time.Millisecond)

	if _, ok := s.Get("k"); ok {
		t.Fatalf("expected key to be expired after waiting")
	}
}
