package once

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// New creates an idempotency middleware with the given store and options.
func New(store Store, opts ...Option) func(http.Handler) http.Handler {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(cfg.header)

			if key == "" {
				if cfg.requireKey {
					http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			if cached, found := store.Get(ctx, key); found {
				if cfg.requestHashCheck {
					requestHash := hashRequestBody(r)
					if cached.RequestHash != "" && cached.RequestHash != requestHash {
						http.Error(w, "Request body mismatch for idempotency key", http.StatusUnprocessableEntity)
						return
					}
				}
				cached.writeTo(w, cfg.replayedHeader)
				return
			}

			unlock, err := store.Lock(ctx, key)
			if err == ErrLocked {
				http.Error(w, "Conflict: request with this idempotency key is in progress", http.StatusConflict)
				return
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			defer unlock()

			var requestHash string
			if cfg.requestHashCheck {
				requestHash = hashRequestBody(r)
			}

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			if cfg.cacheableStatus[rw.statusCode] {
				resp := rw.toResponse(requestHash)
				_ = store.Set(ctx, key, resp, cfg.ttl)
			}
		})
	}
}

func hashRequestBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}

	r.Body = io.NopCloser(readCloserFromBytes(body))

	if len(body) == 0 {
		return ""
	}

	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

type bytesReadCloser struct {
	data []byte
	pos  int
}

func readCloserFromBytes(data []byte) *bytesReadCloser {
	return &bytesReadCloser{data: data}
}

func (b *bytesReadCloser) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReadCloser) Close() error {
	return nil
}
