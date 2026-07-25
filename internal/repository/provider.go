package repository

import (
	"context"

	"github.com/omg/omg/internal/model"
)

// ProviderRepository defines the data-access contract for AI model providers.
// The interface lives here (the producer side) and is consumed by the service layer.
type ProviderRepository interface {
	Create(ctx context.Context, p *model.Provider) error
	GetByID(ctx context.Context, id string) (*model.Provider, error)
	List(ctx context.Context) ([]model.Provider, error)
	Delete(ctx context.Context, id string) error
}
