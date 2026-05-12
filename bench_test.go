package once

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// BenchmarkMemoryStore_Get measures read performance.
func BenchmarkMemoryStore_Get(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()
	key := "bench-key"
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`{"status":"ok"}`),
		CreatedAt:  time.Now(),
	}
	_ = store.Set(ctx, key, resp, time.Hour)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		store.Get(ctx, key)
	}
}

// BenchmarkMemoryStore_Get_Parallel measures concurrent read performance.
func BenchmarkMemoryStore_Get_Parallel(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()
	key := "bench-key"
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`{"status":"ok"}`),
		CreatedAt:  time.Now(),
	}
	_ = store.Set(ctx, key, resp, time.Hour)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.Get(ctx, key)
		}
	})
}

// BenchmarkMemoryStore_Set measures write performance.
func BenchmarkMemoryStore_Set(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`{"status":"ok"}`),
		CreatedAt:  time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = store.Set(ctx, "key", resp, time.Hour)
	}
}

// BenchmarkMemoryStore_Set_UniqueKeys measures write with unique keys.
func BenchmarkMemoryStore_Set_UniqueKeys(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`{"status":"ok"}`),
		CreatedAt:  time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = store.Set(ctx, fmt.Sprintf("key-%d", i), resp, time.Hour)
	}
}

// BenchmarkMemoryStore_Lock measures lock acquisition.
func BenchmarkMemoryStore_Lock(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		unlock, _ := store.Lock(ctx, fmt.Sprintf("key-%d", i))
		unlock()
	}
}

// BenchmarkMemoryStore_LockContention measures lock under contention.
func BenchmarkMemoryStore_LockContention(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()
	keys := make([]string, 100)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := keys[i%len(keys)]
			unlock, err := store.Lock(ctx, key)
			if err == nil {
				unlock()
			}
			i++
		}
	})
}

// BenchmarkMiddleware_CacheHit measures cached response performance.
func BenchmarkMiddleware_CacheHit(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	// Prime the cache
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Idempotency-Key", "cached-key")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("Idempotency-Key", "cached-key")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkMiddleware_CacheHit_Parallel measures concurrent cache hits.
func BenchmarkMiddleware_CacheHit_Parallel(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	// Prime the cache
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Idempotency-Key", "cached-key")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("Idempotency-Key", "cached-key")
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkMiddleware_CacheMiss measures new request processing.
func BenchmarkMiddleware_CacheMiss(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("key-%d", i))
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkMiddleware_NoKey measures passthrough without idempotency key.
func BenchmarkMiddleware_NoKey(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	middleware := New(store)
	wrapped := middleware(handler)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkMiddleware_WithBodyHash measures request body hashing overhead.
func BenchmarkMiddleware_WithBodyHash(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})

	middleware := New(store, WithRequestHashCheck(true))
	wrapped := middleware(handler)

	body := []byte(`{"amount":100,"currency":"USD","recipient":"user123"}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("Idempotency-Key", fmt.Sprintf("key-%d", i))
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkMiddleware_LargeResponse measures large response caching.
func BenchmarkMiddleware_LargeResponse(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	largeBody := bytes.Repeat([]byte("x"), 64*1024) // 64KB

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(largeBody)
	})

	middleware := New(store)
	wrapped := middleware(handler)

	// Prime the cache
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Idempotency-Key", "large-key")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("Idempotency-Key", "large-key")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkThunderingHerd simulates concurrent requests with same key.
func BenchmarkThunderingHerd(b *testing.B) {
	for _, concurrency := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("goroutines-%d", concurrency), func(b *testing.B) {
			store := NewMemoryStore()
			defer store.Stop()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Microsecond) // Simulate work
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			})

			middleware := New(store)
			wrapped := middleware(handler)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				wg.Add(concurrency)

				key := fmt.Sprintf("thunder-%d", i)

				for j := 0; j < concurrency; j++ {
					go func() {
						defer wg.Done()
						req := httptest.NewRequest(http.MethodPost, "/test", nil)
						req.Header.Set("Idempotency-Key", key)
						rec := httptest.NewRecorder()
						wrapped.ServeHTTP(rec, req)
					}()
				}

				wg.Wait()
			}
		})
	}
}

// BenchmarkMemoryStore_MixedWorkload simulates realistic read/write mix.
func BenchmarkMemoryStore_MixedWorkload(b *testing.B) {
	store := NewMemoryStore()
	defer store.Stop()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`{"status":"ok"}`),
			CreatedAt:  time.Now(),
		}
		_ = store.Set(ctx, fmt.Sprintf("existing-%d", i), resp, time.Hour)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				// 10% writes
				resp := &Response{
					StatusCode: 200,
					Body:       []byte(`{"status":"ok"}`),
					CreatedAt:  time.Now(),
				}
				_ = store.Set(ctx, fmt.Sprintf("new-%d", i), resp, time.Hour)
			} else {
				// 90% reads
				store.Get(ctx, fmt.Sprintf("existing-%d", i%1000))
			}
			i++
		}
	})
}
