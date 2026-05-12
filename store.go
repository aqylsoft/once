package once

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrLocked is returned when a key is already locked by another request.
var ErrLocked = errors.New("key is locked")

// Store defines the interface for idempotency key storage.
type Store interface {
	Get(ctx context.Context, key string) (*Response, bool)
	Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error
	Lock(ctx context.Context, key string) (unlock func(), err error)
}

type entry struct {
	resp      *Response
	expiresAt time.Time
}

type keyLock struct {
	mu sync.Mutex
}

// MemoryStore is an in-memory implementation of Store with TTL support.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*entry

	locksMu sync.Mutex
	locks   map[string]*keyLock

	stopCh chan struct{}
	stopWg sync.WaitGroup
}

// NewMemoryStore creates a new in-memory store with background cleanup.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		entries: make(map[string]*entry),
		locks:   make(map[string]*keyLock),
		stopCh:  make(chan struct{}),
	}
	s.stopWg.Add(1)
	go s.cleanupLoop()
	return s
}

// Get retrieves a cached response by key.
func (s *MemoryStore) Get(ctx context.Context, key string) (*Response, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, exists := s.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(e.expiresAt) {
		return nil, false
	}

	return e.resp, true
}

// Set stores a response with the given TTL.
func (s *MemoryStore) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = &entry{
		resp:      resp,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Lock acquires a lock for the given key. Returns ErrLocked if already locked.
func (s *MemoryStore) Lock(ctx context.Context, key string) (func(), error) {
	s.locksMu.Lock()
	kl, exists := s.locks[key]
	if !exists {
		kl = &keyLock{}
		s.locks[key] = kl
	}
	s.locksMu.Unlock()

	if !kl.mu.TryLock() {
		return nil, ErrLocked
	}

	unlock := func() {
		kl.mu.Unlock()
	}
	return unlock, nil
}

// Stop stops the background cleanup goroutine.
func (s *MemoryStore) Stop() {
	close(s.stopCh)
	s.stopWg.Wait()
}

func (s *MemoryStore) cleanupLoop() {
	defer s.stopWg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *MemoryStore) cleanup() {
	now := time.Now()

	s.mu.Lock()
	for key, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, key)
		}
	}
	s.mu.Unlock()
}
