package once

import (
	"bytes"
	"net/http"
	"time"
)

// Response represents a cached HTTP response.
type Response struct {
	StatusCode  int
	Headers     http.Header
	Body        []byte
	RequestHash string
	CreatedAt   time.Time
}

// responseWriter wraps http.ResponseWriter to capture the response.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	written    bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.written {
		return
	}
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
	rw.written = true
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) toResponse(requestHash string) *Response {
	headers := make(http.Header)
	for k, v := range rw.Header() {
		headers[k] = append([]string(nil), v...)
	}

	return &Response{
		StatusCode:  rw.statusCode,
		Headers:     headers,
		Body:        rw.body.Bytes(),
		RequestHash: requestHash,
		CreatedAt:   time.Now(),
	}
}

// writeTo writes the cached response to the given http.ResponseWriter.
func (r *Response) writeTo(w http.ResponseWriter) {
	for k, v := range r.Headers {
		w.Header()[k] = v
	}
	w.WriteHeader(r.StatusCode)
	_, _ = w.Write(r.Body)
}
