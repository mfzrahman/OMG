package model

import "time"

// RateLimit defines rate limiting configuration for a route.
type RateLimit struct {
	ID        string        `json:"id"`
	RouteID   string        `json:"route_id"`
	Requests  int           `json:"requests"`
	Window    time.Duration `json:"window"`
	PerClient bool          `json:"per_client"`
}
