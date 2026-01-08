package store

import (
	"testing"
	"time"
)

func TestReaperDeletesExpiredKeysWithoutAccess(t *testing.T) {
	s := New()

	s.Set("a", []byte("1"))
	if ok := s.Expire("a", 1); !ok {
		t.Fatalf("expected expire to succeed")
	}

	stop := s.StartReaper(50 * time.Millisecond)
	defer stop()

	// Wait long enough for expiry + at least one reaper tick.
	time.Sleep(1200 * time.Millisecond)

	s.mu.RLock()
	_, ok := s.data["a"]
	s.mu.RUnlock()

	if ok {
		t.Fatalf("expected key to be deleted by reaper")
	}
}
