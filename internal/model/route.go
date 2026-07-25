package model

import "time"

// Route represents a configured API route in the gateway.
type Route struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Path           string            `json:"path"`
	Methods        []string          `json:"methods"`
	RewritePattern string            `json:"rewrite_pattern"`
	Headers        map[string]string `json:"headers,omitempty"`
	Timeout        time.Duration     `json:"timeout"`
	Enabled        bool              `json:"enabled"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Backend represents an upstream target for a route.
type Backend struct {
	ID         string `json:"id"`
	RouteID    string `json:"route_id"`
	URL        string `json:"url"`
	Weight     int    `json:"weight"`
	HealthPath string `json:"health_path"`
	Enabled    bool   `json:"enabled"`
}

// BackendWithState includes runtime state for load balancing and circuit
// breaking decisions.
type BackendWithState struct {
	Backend
	ActiveConns  int64                `json:"active_conns"`
	CircuitState CircuitBreakerState  `json:"circuit_state"`
}

// CircuitBreakerState holds the current state of a circuit breaker for a
// single backend.
type CircuitBreakerState struct {
	State       string    `json:"state"` // "closed", "open", "half_open"
	Failures    int       `json:"failures"`
	Successes   int       `json:"successes"`
	LastFailure time.Time `json:"last_failure"`
}
