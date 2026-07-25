package repository

import (
	"context"

	"github.com/omg/omg/internal/model"
)

// RouteRepository is the data-access contract for gateway configuration.
type RouteRepository interface {
	// Routes
	CreateRoute(ctx context.Context, r *model.Route) error
	GetRoute(ctx context.Context, id string) (*model.Route, error)
	ListRoutes(ctx context.Context) ([]model.Route, error)
	UpdateRoute(ctx context.Context, r *model.Route) error
	DeleteRoute(ctx context.Context, id string) error

	// Backends
	CreateBackend(ctx context.Context, b *model.Backend) error
	ListBackends(ctx context.Context, routeID string) ([]model.Backend, error)
	UpdateBackend(ctx context.Context, b *model.Backend) error
	DeleteBackend(ctx context.Context, id string) error

	// Auth
	GetAuthConfig(ctx context.Context, routeID string) (*model.AuthConfig, error)
	SetAuthConfig(ctx context.Context, cfg *model.AuthConfig) error
	DeleteAuthConfig(ctx context.Context, routeID string) error

	// Rate limits
	GetRateLimit(ctx context.Context, routeID string) (*model.RateLimit, error)
	SetRateLimit(ctx context.Context, rl *model.RateLimit) error
	DeleteRateLimit(ctx context.Context, routeID string) error

	// Close releases the database connection.
	Close() error
}
