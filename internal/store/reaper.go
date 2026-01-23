package store

import (
	"sync"
	"time"
)

// StartReaper starts a background goroutine that periodically deletes expired keys.
// It returns a stop() function you MUST call to stop the goroutine.
// stop() is idempotent (safe to call multiple times).
func (s *Store) StartReaper(interval time.Duration) (stop func()) {
	if interval <= 0 {
		interval = 1 * time.Second
	}

	done := make(chan struct{})
	var once sync.Once

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				s.deleteExpired(time.Now())
			case <-done:
				return
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(done)
		})
	}
}

func (s *Store) deleteExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, e := range s.data {
		if isExpired(e, now) {
			delete(s.data, k)
		}
	}
}
