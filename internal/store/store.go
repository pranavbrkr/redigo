package store

import "sync"

type Store struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func New() *Store {
	return &Store{
		data: make(map[string][]byte),
	}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	if !ok {
		return nil, false
	}

	// Return a copy so callers can't mutate internal state
	out := make([]byte, len(val))
	copy(out, val)
	return out, true
}

func (s *Store) Set(key string, val []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy for safety
	cp := make([]byte, len(val))
	copy(cp, val)
	s.data[key] = cp
}

func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, existed := s.data[key]
	if existed {
		delete(s.data, key)
	}
	return existed
}

func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.data[key]
	return ok
}
