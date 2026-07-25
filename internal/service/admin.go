package service

import (
	"context"
	"fmt"
	"time"

	"github.com/omg/omg/internal/model"
	"github.com/omg/omg/internal/repository"
)

// AdminService handles CRUD operations for gateway configuration.
type AdminService struct {
	repo repository.RouteRepository
}

// NewAdmin creates a new AdminService.
func NewAdmin(repo repository.RouteRepository) *AdminService {
	return &AdminService{repo: repo}
}

// ---- Routes ----

func (s *AdminService) CreateRoute(ctx context.Context, rt *model.Route) error {
	if rt.Name == "" {
		return fmt.Errorf("route name is required")
	}
	if rt.Path == "" {
		return fmt.Errorf("route path is required")
	}

	now := time.Now().UTC()
	rt.ID = generateID("rt")
	rt.CreatedAt = now
	rt.UpdatedAt = now
	if rt.Timeout == 0 {
		rt.Timeout = 30 * time.Second
	}
	if len(rt.Methods) == 0 {
		rt.Methods = []string{"GET"}
	}

	return s.repo.CreateRoute(ctx, rt)
}

func (s *AdminService) GetRoute(ctx context.Context, id string) (*model.Route, error) {
	return s.repo.GetRoute(ctx, id)
}

func (s *AdminService) ListRoutes(ctx context.Context) ([]model.Route, error) {
	return s.repo.ListRoutes(ctx)
}

func (s *AdminService) UpdateRoute(ctx context.Context, rt *model.Route) error {
	rt.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateRoute(ctx, rt)
}

func (s *AdminService) DeleteRoute(ctx context.Context, id string) error {
	return s.repo.DeleteRoute(ctx, id)
}

// ---- Backends ----

func (s *AdminService) AddBackend(ctx context.Context, b *model.Backend) error {
	if b.URL == "" {
		return fmt.Errorf("backend url is required")
	}
	b.ID = generateID("be")
	return s.repo.CreateBackend(ctx, b)
}

func (s *AdminService) ListBackends(ctx context.Context, routeID string) ([]model.Backend, error) {
	return s.repo.ListBackends(ctx, routeID)
}

func (s *AdminService) UpdateBackend(ctx context.Context, b *model.Backend) error {
	return s.repo.UpdateBackend(ctx, b)
}

func (s *AdminService) RemoveBackend(ctx context.Context, id string) error {
	return s.repo.DeleteBackend(ctx, id)
}

// ---- Auth ----

func (s *AdminService) GetAuthConfig(ctx context.Context, routeID string) (*model.AuthConfig, error) {
	return s.repo.GetAuthConfig(ctx, routeID)
}

func (s *AdminService) SetAuthConfig(ctx context.Context, cfg *model.AuthConfig) error {
	if cfg.Type != "jwt" && cfg.Type != "api_key" {
		return fmt.Errorf("auth type must be 'jwt' or 'api_key'")
	}
	cfg.ID = generateID("auth")
	return s.repo.SetAuthConfig(ctx, cfg)
}

func (s *AdminService) RemoveAuthConfig(ctx context.Context, routeID string) error {
	return s.repo.DeleteAuthConfig(ctx, routeID)
}

// ---- Rate Limits ----

func (s *AdminService) GetRateLimit(ctx context.Context, routeID string) (*model.RateLimit, error) {
	return s.repo.GetRateLimit(ctx, routeID)
}

func (s *AdminService) SetRateLimit(ctx context.Context, rl *model.RateLimit) error {
	if rl.Requests <= 0 {
		return fmt.Errorf("rate limit requests must be > 0")
	}
	if rl.Window <= 0 {
		return fmt.Errorf("rate limit window must be > 0")
	}
	rl.ID = generateID("rl")
	return s.repo.SetRateLimit(ctx, rl)
}

func (s *AdminService) RemoveRateLimit(ctx context.Context, routeID string) error {
	return s.repo.DeleteRateLimit(ctx, routeID)
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
