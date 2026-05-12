# once

Idempotency key middleware for Go. Prevents duplicate request processing by caching responses.

## Features

- Standard `net/http` middleware
- In-memory store with TTL
- Per-key locking (no thundering herd)
- Request body hash validation
- C shared library via cgo

## Installation

```bash
go get github.com/aqylsoft/once
```

## Usage

```go
package main

import (
    "net/http"
    "time"

    "github.com/aqylsoft/once"
)

func main() {
    store := once.NewMemoryStore()
    defer store.Stop()

    middleware := once.New(store,
        once.WithTTL(1*time.Hour),
        once.WithRequireKey(true),
    )

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"created"}`))
    })

    http.Handle("/payments", middleware(handler))
    http.ListenAndServe(":8080", nil)
}
```

Client sends `Idempotency-Key` header:

```bash
curl -X POST http://localhost:8080/payments \
  -H "Idempotency-Key: unique-request-id" \
  -d '{"amount": 100}'
```

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithHeader(name)` | `Idempotency-Key` | Header name for idempotency key |
| `WithTTL(duration)` | `24h` | Cache TTL |
| `WithRequireKey(bool)` | `false` | Return 400 if key missing |
| `WithCacheableStatus(codes...)` | `200, 201, 204` | Status codes to cache |
| `WithRequestHashCheck(bool)` | `false` | Validate request body hash |

## Custom Store

Implement the `Store` interface for Redis, PostgreSQL, etc:

```go
type Store interface {
    Get(ctx context.Context, key string) (*Response, bool)
    Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error
    Lock(ctx context.Context, key string) (unlock func(), err error)
}
```

## C Library

Build shared library:

```bash
make c-shared
```

Outputs `cgo/libonce.so` and `cgo/libonce.h`.

### C API

```c
void once_init(void);
void once_destroy(void);
int  once_check(char* key, char* response, int responseLen);
int  once_store(char* key, int statusCode, char* body, int bodyLen, int ttlSeconds);
int  once_lock(char* key);
void once_unlock(int lockId);
```

## HTTP Response Codes

| Code | Meaning |
|------|---------|
| 200 | Cached response returned |
| 400 | Missing required idempotency key |
| 409 | Request with same key in progress |
| 422 | Request body mismatch |

## Benchmarks

Run benchmarks:

```bash
make bench
```

Results on Intel Core i7-1355U:

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| MemoryStore_Get | 50 | 0 | 0 |
| MemoryStore_Get_Parallel | 89 | 0 | 0 |
| MemoryStore_Set | 116 | 32 | 1 |
| MemoryStore_Lock | 420 | 132 | 4 |
| Middleware_CacheHit | 2,834 | 5,747 | 16 |
| Middleware_CacheHit_Parallel | 3,427 | 5,750 | 16 |
| Middleware_CacheMiss | 4,429 | 6,307 | 26 |
| Middleware_NoKey (passthrough) | 2,716 | 5,395 | 15 |
| Middleware_WithBodyHash | 4,632 | 7,048 | 33 |
| ThunderingHerd (100 goroutines) | 618,231 | 617,004 | 1,990 |
| MixedWorkload (90% read/10% write) | 483 | 35 | 2 |

## License

MIT
