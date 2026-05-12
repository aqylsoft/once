package once

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMiddleware_NoKey(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected handler to be called once, got %d", callCount.Load())
	}
}

func TestMiddleware_RequireKey(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := New(store, WithRequireKey(true))
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestMiddleware_IdempotentRequest(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"success"}`))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	key := "unique-key-123"

	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", rec1.Code)
	}

	if callCount.Load() != 1 {
		t.Errorf("first request: expected handler called once, got %d", callCount.Load())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("second request: expected status 200, got %d", rec2.Code)
	}

	if callCount.Load() != 1 {
		t.Errorf("second request: handler should not be called again, got %d calls", callCount.Load())
	}

	if rec2.Body.String() != `{"result":"success"}` {
		t.Errorf("second request: expected cached body, got %s", rec2.Body.String())
	}

	if rec2.Header().Get("X-Custom") != "value" {
		t.Errorf("second request: expected cached header, got %s", rec2.Header().Get("X-Custom"))
	}
}

func TestMiddleware_ThunderingHerd(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	const goroutines = 100
	key := "thundering-herd-key"

	var wg sync.WaitGroup
	wg.Add(goroutines)

	var successCount atomic.Int32
	var conflictCount atomic.Int32

	start := make(chan struct{})

	for range goroutines {
		go func() {
			defer wg.Done()
			<-start

			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("Idempotency-Key", key)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				successCount.Add(1)
			case http.StatusConflict:
				conflictCount.Add(1)
			}
		}()
	}

	close(start)
	wg.Wait()

	if callCount.Load() != 1 {
		t.Errorf("expected handler called exactly once, got %d", callCount.Load())
	}

	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount.Load())
	}

	if conflictCount.Load() != goroutines-1 {
		t.Errorf("expected %d conflicts, got %d", goroutines-1, conflictCount.Load())
	}
}

func TestMiddleware_BodyMismatch(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	middleware := New(store, WithRequestHashCheck(true))
	wrapped := middleware(handler)

	key := "body-check-key"

	req1 := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(`{"amount":100}`)))
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(`{"amount":200}`)))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnprocessableEntity {
		t.Errorf("second request with different body: expected status 422, got %d", rec2.Code)
	}
}

func TestMiddleware_SameBodyOK(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := New(store, WithRequestHashCheck(true))
	wrapped := middleware(handler)

	key := "same-body-key"
	body := `{"amount":100}`

	req1 := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(body)))
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(body)))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("second request with same body: expected status 200, got %d", rec2.Code)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected handler called once, got %d", callCount.Load())
	}
}

func TestMiddleware_CustomHeader(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	})

	middleware := New(store, WithHeader("X-Request-ID"))
	wrapped := middleware(handler)

	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req1.Header.Set("X-Request-ID", "custom-key")
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("X-Request-ID", "custom-key")
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if callCount.Load() != 1 {
		t.Errorf("expected handler called once with custom header, got %d", callCount.Load())
	}
}

func TestMiddleware_NonCacheableStatus(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	key := "error-key"

	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice for non-cacheable status, got %d", callCount.Load())
	}
}

func TestMiddleware_CustomCacheableStatus(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("accepted"))
	})

	middleware := New(store, WithCacheableStatus(202))
	wrapped := middleware(handler)

	key := "accepted-key"

	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if callCount.Load() != 1 {
		t.Errorf("expected handler called once with custom cacheable status, got %d", callCount.Load())
	}
}

func TestMiddleware_TTL(t *testing.T) {
	store := NewMemoryStore()
	defer store.Stop()

	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := New(store, WithTTL(50*time.Millisecond))
	wrapped := middleware(handler)

	key := "ttl-key"

	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec1, req1)

	if callCount.Load() != 1 {
		t.Errorf("first request: expected handler called once, got %d", callCount.Load())
	}

	time.Sleep(100 * time.Millisecond)

	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()

	wrapped.ServeHTTP(rec2, req2)

	if callCount.Load() != 2 {
		t.Errorf("after TTL expiry: expected handler called twice, got %d", callCount.Load())
	}
}
