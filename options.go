package once

import "time"

const (
	defaultHeader = "Idempotency-Key"
	defaultTTL    = 24 * time.Hour
)

var defaultCacheableStatus = map[int]bool{
	200: true,
	201: true,
	204: true,
}

type config struct {
	header           string
	ttl              time.Duration
	requireKey       bool
	cacheableStatus  map[int]bool
	requestHashCheck bool
}

func defaultConfig() *config {
	cacheable := make(map[int]bool)
	for k, v := range defaultCacheableStatus {
		cacheable[k] = v
	}

	return &config{
		header:           defaultHeader,
		ttl:              defaultTTL,
		requireKey:       false,
		cacheableStatus:  cacheable,
		requestHashCheck: false,
	}
}

// Option configures the idempotency middleware.
type Option func(*config)

// WithHeader sets the header name for the idempotency key.
// Default: "Idempotency-Key"
func WithHeader(name string) Option {
	return func(c *config) {
		c.header = name
	}
}

// WithTTL sets the time-to-live for cached responses.
// Default: 24 hours
func WithTTL(d time.Duration) Option {
	return func(c *config) {
		c.ttl = d
	}
}

// WithRequireKey sets whether the idempotency key is required.
// If true, requests without a key will receive a 400 Bad Request response.
// Default: false
func WithRequireKey(required bool) Option {
	return func(c *config) {
		c.requireKey = required
	}
}

// WithCacheableStatus sets which HTTP status codes should be cached.
// Default: 200, 201, 204
func WithCacheableStatus(codes ...int) Option {
	return func(c *config) {
		c.cacheableStatus = make(map[int]bool)
		for _, code := range codes {
			c.cacheableStatus[code] = true
		}
	}
}

// WithRequestHashCheck enables request body hash checking.
// When enabled, if a request with the same idempotency key but different body
// is received, a 422 Unprocessable Entity response is returned.
// Default: false
func WithRequestHashCheck(enabled bool) Option {
	return func(c *config) {
		c.requestHashCheck = enabled
	}
}
