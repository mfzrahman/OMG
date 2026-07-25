package service

import (
	"context"
	"fmt"
	"time"

	"github.com/omg/omg/internal/model"
	"github.com/omg/omg/internal/repository"
)

// ProviderService contains business logic for managing AI model providers.
type ProviderService struct {
	repo repository.ProviderRepository
}

// NewProvider creates a new ProviderService.
func NewProvider(repo repository.ProviderRepository) *ProviderService {
	return &ProviderService{repo: repo}
}

// Register adds a new provider to the gateway.
func (s *ProviderService) Register(ctx context.Context, req model.CreateProviderRequest) (*model.CreateProviderResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	if req.BaseURL == "" {
		return nil, fmt.Errorf("provider base_url is required")
	}

	now := time.Now().UTC()
	p := &model.Provider{
		ID:        generateID(),
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("register provider: %w", err)
	}

	return &model.CreateProviderResponse{
		ID:        p.ID,
		Name:      p.Name,
		BaseURL:   p.BaseURL,
		CreatedAt: p.CreatedAt,
	}, nil
}

// GetByID retrieves a single provider.
func (s *ProviderService) GetByID(ctx context.Context, id string) (*model.Provider, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns all registered providers.
func (s *ProviderService) List(ctx context.Context) ([]model.Provider, error) {
	return s.repo.List(ctx)
}

// Remove deletes a provider by id.
func (s *ProviderService) Remove(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// generateID produces a simple unique identifier. Replace with ULID or UUID
// for production use.
func generateID() string {
	return fmt.Sprintf("prv_%d", time.Now().UnixNano())
}
