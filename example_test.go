package once_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/aqylsoft/once"
)

func ExampleNew() {
	store := once.NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})

	middleware := once.New(store)
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/payments", nil)
	req.Header.Set("Idempotency-Key", "unique-payment-id")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)
	fmt.Println(rec.Code)
	// Output: 200
}

func ExampleNew_withOptions() {
	store := once.NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})

	middleware := once.New(store,
		once.WithHeader("X-Idempotency-Key"),
		once.WithTTL(1*time.Hour),
		once.WithRequireKey(true),
		once.WithRequestHashCheck(true),
		once.WithCacheableStatus(200, 201, 202),
	)

	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("X-Idempotency-Key", "order-123")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)
	fmt.Println(rec.Code)
	// Output: 201
}

func ExampleWithRequireKey() {
	store := once.NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := once.New(store, once.WithRequireKey(true))
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/payments", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)
	fmt.Println(rec.Code)
	// Output: 400
}

func ExampleMemoryStore() {
	store := once.NewMemoryStore()
	defer store.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Handler called")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	middleware := once.New(store)
	wrapped := middleware(handler)

	for i := range 3 {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("Idempotency-Key", "same-key")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		fmt.Printf("Request %d: status=%d\n", i+1, rec.Code)
	}

	// Output:
	// Handler called
	// Request 1: status=200
	// Request 2: status=200
	// Request 3: status=200
}
