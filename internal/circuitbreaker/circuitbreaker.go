package circuitbreaker

import (
	"sync"
	"time"

	"github.com/omg/omg/internal/model"
)

// CircuitBreaker implements the circuit breaker pattern for backend
// resilience. State transitions: closed → open → half_open → closed.
type CircuitBreaker struct {
	mu          sync.RWMutex
	states      map[string]*breakerState
	threshold   int
	cooldown    time.Duration
	halfOpenMax int
}

type breakerState struct {
	state       string
	failures    int
	successes   int
	lastFailure time.Time
}

// New creates a new CircuitBreaker.
func New(threshold int, cooldown time.Duration, halfOpenMax int) *CircuitBreaker {
	return &CircuitBreaker{
		states:      make(map[string]*breakerState),
		threshold:   threshold,
		cooldown:    cooldown,
		halfOpenMax: halfOpenMax,
	}
}

// Allow returns true if the backend may be used. Open circuits are
// allowed a single probe after the cooldown expires (half-open state).
func (cb *CircuitBreaker) Allow(backendID string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	bs := cb.ensureState(backendID)
	switch bs.state {
	case "closed":
		return true
	case "open":
		if time.Since(bs.lastFailure) > cb.cooldown {
			bs.state = "half_open"
			bs.successes = 0
			return true
		}
		return false
	case "half_open":
		return bs.successes < cb.halfOpenMax
	default:
		return true
	}
}

// Success records a successful request and closes the circuit if
// half-open.
func (cb *CircuitBreaker) Success(backendID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	bs := cb.ensureState(backendID)
	bs.failures = 0
	if bs.state == "half_open" {
		bs.successes++
		if bs.successes >= cb.halfOpenMax {
			bs.state = "closed"
		}
	}
}

// Failure records a failed request. If failures exceed the threshold
// the circuit opens.
func (cb *CircuitBreaker) Failure(backendID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	bs := cb.ensureState(backendID)
	bs.failures++
	bs.lastFailure = time.Now()
	if bs.failures >= cb.threshold || bs.state == "half_open" {
		bs.state = "open"
	}
}

// State returns the current circuit state for a backend.
func (cb *CircuitBreaker) State(backendID string) model.CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	bs := cb.ensureState(backendID)
	return model.CircuitBreakerState{
		State:       bs.state,
		Failures:    bs.failures,
		Successes:   bs.successes,
		LastFailure: bs.lastFailure,
	}
}

func (cb *CircuitBreaker) ensureState(id string) *breakerState {
	if bs, ok := cb.states[id]; ok {
		return bs
	}
	bs := &breakerState{state: "closed"}
	cb.states[id] = bs
	return bs
}
