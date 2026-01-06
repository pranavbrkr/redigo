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
	defer s.mu.RUnlock()

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

func isExpired(e entry, now time.Time) bool {
	if e.expiresAt == nil {
		return false
	}
	return !now.Before(*e.expiresAt)
}
