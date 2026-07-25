package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/omg/omg/internal/model"
)

type routeSQLite struct {
	db *sql.DB
}

// NewRouteSQLite returns a SQLite-backed RouteRepository.
func NewRouteSQLite(db *sql.DB) RouteRepository {
	return &routeSQLite{db: db}
}

// ---- Routes ----

func (r *routeSQLite) CreateRoute(ctx context.Context, rt *model.Route) error {
	methods, _ := json.Marshal(rt.Methods)
	headers, _ := json.Marshal(rt.Headers)
	const query = `
		INSERT INTO routes (id, name, path, methods, rewrite_pattern, headers, timeout_ns, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		rt.ID, rt.Name, rt.Path, string(methods), rt.RewritePattern,
		string(headers), rt.Timeout.Nanoseconds(), rt.Enabled,
		rt.CreatedAt.UnixMilli(), rt.UpdatedAt.UnixMilli(),
	)
	return err
}

func (r *routeSQLite) GetRoute(ctx context.Context, id string) (*model.Route, error) {
	const query = `
		SELECT id, name, path, methods, rewrite_pattern, headers, timeout_ns, enabled, created_at, updated_at
		FROM routes WHERE id = ?`
	rt := &model.Route{}
	var methods, headers string
	var timeoutNs, createdAtMs, updatedAtMs int64
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rt.ID, &rt.Name, &rt.Path, &methods, &rt.RewritePattern,
		&headers, &timeoutNs, &rt.Enabled, &createdAtMs, &updatedAtMs,
	)
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}
	json.Unmarshal([]byte(methods), &rt.Methods)
	json.Unmarshal([]byte(headers), &rt.Headers)
	rt.Timeout = time.Duration(timeoutNs)
	rt.CreatedAt = time.UnixMilli(createdAtMs)
	rt.UpdatedAt = time.UnixMilli(updatedAtMs)
	return rt, nil
}

func (r *routeSQLite) ListRoutes(ctx context.Context) ([]model.Route, error) {
	const query = `
		SELECT id, name, path, methods, rewrite_pattern, headers, timeout_ns, enabled, created_at, updated_at
		FROM routes ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var routes []model.Route
	for rows.Next() {
		var rt model.Route
		var methods, headers string
		var timeoutNs, createdAtMs, updatedAtMs int64
		if err := rows.Scan(
			&rt.ID, &rt.Name, &rt.Path, &methods, &rt.RewritePattern,
			&headers, &timeoutNs, &rt.Enabled, &createdAtMs, &updatedAtMs,
		); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		json.Unmarshal([]byte(methods), &rt.Methods)
		json.Unmarshal([]byte(headers), &rt.Headers)
		rt.Timeout = time.Duration(timeoutNs)
		rt.CreatedAt = time.UnixMilli(createdAtMs)
		rt.UpdatedAt = time.UnixMilli(updatedAtMs)
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}

func (r *routeSQLite) UpdateRoute(ctx context.Context, rt *model.Route) error {
	methods, _ := json.Marshal(rt.Methods)
	headers, _ := json.Marshal(rt.Headers)
	const query = `
		UPDATE routes SET name=?, path=?, methods=?, rewrite_pattern=?, headers=?, timeout_ns=?, enabled=?, updated_at=?
		WHERE id=?`
	_, err := r.db.ExecContext(ctx, query,
		rt.Name, rt.Path, string(methods), rt.RewritePattern,
		string(headers), rt.Timeout.Nanoseconds(), rt.Enabled, rt.UpdatedAt.UnixMilli(), rt.ID,
	)
	return err
}

func (r *routeSQLite) DeleteRoute(ctx context.Context, id string) error {
	r.db.ExecContext(ctx, "DELETE FROM backends WHERE route_id=?", id)
	r.db.ExecContext(ctx, "DELETE FROM auth_configs WHERE route_id=?", id)
	r.db.ExecContext(ctx, "DELETE FROM rate_limits WHERE route_id=?", id)
	_, err := r.db.ExecContext(ctx, "DELETE FROM routes WHERE id=?", id)
	return err
}

// ---- Backends ----

func (r *routeSQLite) CreateBackend(ctx context.Context, b *model.Backend) error {
	const query = `
		INSERT INTO backends (id, route_id, url, weight, health_path, enabled)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, b.ID, b.RouteID, b.URL, b.Weight, b.HealthPath, b.Enabled)
	return err
}

func (r *routeSQLite) ListBackends(ctx context.Context, routeID string) ([]model.Backend, error) {
	const query = `
		SELECT id, route_id, url, weight, health_path, enabled
		FROM backends WHERE route_id=? AND enabled=1 ORDER BY weight DESC`
	rows, err := r.db.QueryContext(ctx, query, routeID)
	if err != nil {
		return nil, fmt.Errorf("list backends: %w", err)
	}
	defer rows.Close()

	var backends []model.Backend
	for rows.Next() {
		var b model.Backend
		if err := rows.Scan(&b.ID, &b.RouteID, &b.URL, &b.Weight, &b.HealthPath, &b.Enabled); err != nil {
			return nil, fmt.Errorf("scan backend: %w", err)
		}
		backends = append(backends, b)
	}
	return backends, rows.Err()
}

func (r *routeSQLite) UpdateBackend(ctx context.Context, b *model.Backend) error {
	const query = `
		UPDATE backends SET url=?, weight=?, health_path=?, enabled=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, b.URL, b.Weight, b.HealthPath, b.Enabled, b.ID)
	return err
}

func (r *routeSQLite) DeleteBackend(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM backends WHERE id=?", id)
	return err
}

// ---- Auth Configs ----

func (r *routeSQLite) GetAuthConfig(ctx context.Context, routeID string) (*model.AuthConfig, error) {
	const query = `
		SELECT id, route_id, type, issuer, secret, api_key FROM auth_configs WHERE route_id=?`
	cfg := &model.AuthConfig{}
	err := r.db.QueryRowContext(ctx, query, routeID).Scan(
		&cfg.ID, &cfg.RouteID, &cfg.Type, &cfg.Issuer, &cfg.Secret, &cfg.APIKey,
	)
	if err != nil {
		return nil, fmt.Errorf("get auth config: %w", err)
	}
	return cfg, nil
}

func (r *routeSQLite) SetAuthConfig(ctx context.Context, cfg *model.AuthConfig) error {
	r.db.ExecContext(ctx, "DELETE FROM auth_configs WHERE route_id=?", cfg.RouteID)
	const query = `
		INSERT INTO auth_configs (id, route_id, type, issuer, secret, api_key)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, cfg.ID, cfg.RouteID, cfg.Type, cfg.Issuer, cfg.Secret, cfg.APIKey)
	return err
}

func (r *routeSQLite) DeleteAuthConfig(ctx context.Context, routeID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM auth_configs WHERE route_id=?", routeID)
	return err
}

// ---- Rate Limits ----

func (r *routeSQLite) GetRateLimit(ctx context.Context, routeID string) (*model.RateLimit, error) {
	const query = `
		SELECT id, route_id, requests, window_ns, per_client FROM rate_limits WHERE route_id=?`
	rl := &model.RateLimit{}
	var windowNs int64
	err := r.db.QueryRowContext(ctx, query, routeID).Scan(
		&rl.ID, &rl.RouteID, &rl.Requests, &windowNs, &rl.PerClient,
	)
	if err != nil {
		return nil, fmt.Errorf("get rate limit: %w", err)
	}
	rl.Window = time.Duration(windowNs)
	return rl, nil
}

func (r *routeSQLite) SetRateLimit(ctx context.Context, rl *model.RateLimit) error {
	r.db.ExecContext(ctx, "DELETE FROM rate_limits WHERE route_id=?", rl.RouteID)
	const query = `
		INSERT INTO rate_limits (id, route_id, requests, window_ns, per_client)
		VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, rl.ID, rl.RouteID, rl.Requests, rl.Window.Nanoseconds(), rl.PerClient)
	return err
}

func (r *routeSQLite) DeleteRateLimit(ctx context.Context, routeID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM rate_limits WHERE route_id=?", routeID)
	return err
}

func (r *routeSQLite) Close() error {
	return r.db.Close()
}
