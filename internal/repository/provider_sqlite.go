package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/omg/omg/internal/model"
)

// providerSQLite is the SQLite implementation of ProviderRepository.
type providerSQLite struct {
	db *sql.DB
}

// NewProviderSQLite returns a SQLite-backed ProviderRepository.
func NewProviderSQLite(db *sql.DB) ProviderRepository {
	return &providerSQLite{db: db}
}

func (r *providerSQLite) Create(ctx context.Context, p *model.Provider) error {
	const query = `
		INSERT INTO providers (id, name, base_url, api_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Name, p.BaseURL, p.APIKey, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert provider: %w", err)
	}
	return nil
}

func (r *providerSQLite) GetByID(ctx context.Context, id string) (*model.Provider, error) {
	const query = `
		SELECT id, name, base_url, api_key, created_at, updated_at
		FROM providers WHERE id = ?`
	p := &model.Provider{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get provider by id: %w", err)
	}
	return p, nil
}

func (r *providerSQLite) List(ctx context.Context) ([]model.Provider, error) {
	const query = `
		SELECT id, name, base_url, api_key, created_at, updated_at
		FROM providers ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []model.Provider
	for rows.Next() {
		var p model.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func (r *providerSQLite) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM providers WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}
