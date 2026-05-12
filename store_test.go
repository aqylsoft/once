package once

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryStore_GetSet(t *testing.T) {
	s := NewMemoryStore()
	defer s.Stop()

	ctx := context.Background()
	key := "test-key"

	_, found := s.Get(ctx, key)
	if found {
		t.Error("expected not found for non-existent key")
	}

	resp := &Response{
		StatusCode:  200,
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		Body:        []byte(`{"status":"ok"}`),
		RequestHash: "abc123",
		CreatedAt:   time.Now(),
	}

	err := s.Set(ctx, key, resp, time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, found := s.Get(ctx, key)
	if !found {
		t.Fatal("expected to find key after Set")
	}

	if got.StatusCode != resp.StatusCode {
		t.Errorf("StatusCode = %d, want %d", got.StatusCode, resp.StatusCode)
	}

	if string(got.Body) != string(resp.Body) {
		t.Errorf("Body = %s, want %s", got.Body, resp.Body)
	}

	if got.RequestHash != resp.RequestHash {
		t.Errorf("RequestHash = %s, want %s", got.RequestHash, resp.RequestHash)
	}
}

func TestMemoryStore_TTLExpiration(t *testing.T) {
	s := NewMemoryStore()
	defer s.Stop()

	ctx := context.Background()
	key := "expiring-key"

	resp := &Response{
		StatusCode: 200,
		Body:       []byte("test"),
		CreatedAt:  time.Now(),
	}

	err := s.Set(ctx, key, resp, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	_, found := s.Get(ctx, key)
	if !found {
		t.Error("expected to find key immediately after Set")
	}

	time.Sleep(100 * time.Millisecond)

	_, found = s.Get(ctx, key)
	if found {
		t.Error("expected key to be expired")
	}
}

func TestMemoryStore_Lock(t *testing.T) {
	s := NewMemoryStore()
	defer s.Stop()

	ctx := context.Background()
	key := "lock-key"

	unlock, err := s.Lock(ctx, key)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	_, err = s.Lock(ctx, key)
	if err != ErrLocked {
		t.Errorf("expected ErrLocked, got %v", err)
	}

	unlock()

	unlock2, err := s.Lock(ctx, key)
	if err != nil {
		t.Fatalf("Lock after unlock failed: %v", err)
	}
	unlock2()
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	s := NewMemoryStore()
	defer s.Stop()

	ctx := context.Background()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			resp := &Response{
				StatusCode: 200,
				Body:       []byte("test"),
				CreatedAt:  time.Now(),
			}
			_ = s.Set(ctx, "key", resp, time.Hour)
		}(i)

		go func(i int) {
			defer wg.Done()
			s.Get(ctx, "key")
		}(i)
	}

	wg.Wait()
}

func TestMemoryStore_LockThunderingHerd(t *testing.T) {
	s := NewMemoryStore()
	defer s.Stop()

	ctx := context.Background()
	key := "thundering-herd-key"
	const goroutines = 100

	var successCount atomic.Int32
	var lockedCount atomic.Int32

	var wg sync.WaitGroup
	wg.Add(goroutines)

	start := make(chan struct{})

	for range goroutines {
		go func() {
			defer wg.Done()
			<-start

			unlock, err := s.Lock(ctx, key)
			if err == ErrLocked {
				lockedCount.Add(1)
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			successCount.Add(1)
			time.Sleep(10 * time.Millisecond)
			unlock()
		}()
	}

	close(start)
	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 successful lock, got %d", successCount.Load())
	}

	if lockedCount.Load() != goroutines-1 {
		t.Errorf("expected %d locked, got %d", goroutines-1, lockedCount.Load())
	}
}

func TestMemoryStore_Stop(t *testing.T) {
	s := NewMemoryStore()
	s.Stop()
}
