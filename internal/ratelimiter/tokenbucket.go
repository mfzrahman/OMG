package ratelimiter

import (
	"context"
	"sync"
	"time"

	"github.com/omg/omg/internal/model"
)

// TokenBucket implements a lazy-refill token bucket rate limiter.
// Refill happens on each Allow() call rather than in a background
// goroutine.
type TokenBucket struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	lastFill time.Time
}

// NewTokenBucket creates a new token bucket rate limiter.
func NewTokenBucket() *TokenBucket {
	return &TokenBucket{
		buckets: make(map[string]*bucket),
	}
}

// Allow checks if a request identified by key should be allowed under
// the given rate limit. Returns true if allowed.
func (tb *TokenBucket) Allow(ctx context.Context, key string, limit *model.RateLimit) (bool, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, ok := tb.buckets[key]
	now := time.Now()

	if !ok {
		b = &bucket{tokens: float64(limit.Requests) - 1, lastFill: now}
		tb.buckets[key] = b
		return true, nil
	}

	// Lazy refill — add tokens based on elapsed time.
	elapsed := now.Sub(b.lastFill).Seconds()
	rate := float64(limit.Requests) / limit.Window.Seconds()
	b.tokens += elapsed * rate
	if b.tokens > float64(limit.Requests) {
		b.tokens = float64(limit.Requests)
	}
	b.lastFill = now

	if b.tokens >= 1 {
		b.tokens--
		return true, nil
	}

	return false, nil
}

// Cleanup removes expired bucket entries. Call periodically to prevent
// memory leaks.
func (tb *TokenBucket) Cleanup(maxAge time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	for k, b := range tb.buckets {
		if now.Sub(b.lastFill) > maxAge {
			delete(tb.buckets, k)
		}
	}
}
