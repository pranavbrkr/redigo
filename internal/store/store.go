package store

import (
	"sync"
	"time"
)

type entry struct {
	value     []byte
	expiresAt *time.Time // nil means no expiration
}

type Store struct {
	mu   sync.RWMutex
	data map[string]entry
}

func New() *Store {
	return &Store{
		data: make(map[string]entry),
	}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return nil, false
	}

	if isExpired(e, time.Now()) {
		delete(s.data, key)
		return nil, false
	}

	// Return a copy so callers can't mutate internal state
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, true
}

func (s *Store) Set(key string, val []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy for safety
	cp := make([]byte, len(val))
	copy(cp, val)

	// SET clears any existing expiry
	s.data[key] = entry{
		value:     cp,
		expiresAt: nil,
	}
}

func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return false
	}

	// If it's expired. treat it as already gone
	if isExpired(e, time.Now()) {
		delete(s.data, key)
		return false
	}

	delete(s.data, key)
	return true
}

func (s *Store) Exists(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return false
	}

	if isExpired(e, time.Now()) {
		delete(s.data, key)
		return false
	}

	return true
}

// Expire sets an expiration on key for given number of seconds
// Returns true if key exists and expiry was set, false otherwise
func (s *Store) Expire(key string, seconds int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return false
	}

	now := time.Now()
	if isExpired(e, now) {
		delete(s.data, key)
		return false
	}

	if seconds <= 0 {
		delete(s.data, key)
		return true
	}

	exp := now.Add(time.Duration(seconds) * time.Second)
	e.expiresAt = &exp
	s.data[key] = e
	return true
}

// TTL returns Redis-like TTL semantics
// -2 if key does not exist
// -1 if key exists but has no expiry
// >=0 remaining seconds otherwise
func (s *Store) TTL(key string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return -2
	}

	now := time.Now()
	if isExpired(e, now) {
		delete(s.data, key)
		return -2
	}

	if e.expiresAt == nil {
		return -1
	}

	remaining := e.expiresAt.Sub(now)
	sec := int64(remaining / time.Second)
	// Not needed may be
	// if sec < 0 {
	// 	delete(s.data, key)
	// 	return -2
	// }
	return sec
}

func isExpired(e entry, now time.Time) bool {
	if e.expiresAt == nil {
		return false
	}
	return !now.Before(*e.expiresAt)
}

// ExpireAt sets an absolute expiration time on key (unix timestamp in seconds).
// Returns true if key exists and expiry was set (or key deleted due to past timestamp), false otherwise.
func (s *Store) ExpireAt(key string, unixSeconds int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return false
	}

	now := time.Now()
	if isExpired(e, now) {
		delete(s.data, key)
		return false
	}

	exp := time.Unix(unixSeconds, 0)

	// Redis semantics: if timestamp is in the past (or now), the key is deleted and return 1 (since it existed)
	if !now.Before(exp) {
		delete(s.data, key)
		return true
	}

	e.expiresAt = &exp
	s.data[key] = e
	return true
}
