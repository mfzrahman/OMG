package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// New generates a random 16-byte hex request ID.
func New() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// FromContext extracts the request ID from the context.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	if id == "" {
		return "unknown"
	}
	return id
}

// WithContext stores the request ID in the context.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// FromRequest extracts the request ID from the X-Request-Id header or
// generates a new one.
func FromRequest(r *http.Request) string {
	id := r.Header.Get("X-Request-Id")
	if id == "" {
		id = New()
	}
	return id
}
