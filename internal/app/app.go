package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/omg/omg/internal/auth"
	"github.com/omg/omg/internal/circuitbreaker"
	"github.com/omg/omg/internal/config"
	"github.com/omg/omg/internal/handler"
	"github.com/omg/omg/internal/loadbalancer"
	"github.com/omg/omg/internal/metrics"
	"github.com/omg/omg/internal/ratelimiter"
	"github.com/omg/omg/internal/repository"
	"github.com/omg/omg/internal/service"

	_ "modernc.org/sqlite"
)

// App is the dependency container. It holds every wired component.
type App struct {
	Config       *config.Config
	AdminHandler *handler.AdminHandler
	ProxyHandler *handler.ProxyHandler
	Metrics      *metrics.Collector
	Repo         repository.RouteRepository
}

// New wires the full application and returns it with a cleanup function.
func New(cfg *config.Config) (*App, func(), error) {
	// Ensure parent directory exists for the database file.
	if dir := filepath.Dir(cfg.DatabasePath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("db open: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, nil, fmt.Errorf("db ping: %w", err)
	}

	// Run migrations.
	if err := runMigrations(db); err != nil {
		return nil, nil, fmt.Errorf("migrations: %w", err)
	}

	// --- Wire dependencies ---
	repo := repository.NewRouteSQLite(db)

	adminSvc := service.NewAdmin(repo)
	adminH := handler.NewAdmin(adminSvc)

	gatewaySvc := service.NewGateway(repo)

	lb := loadbalancer.NewRoundRobin()

	breaker := circuitbreaker.New(5, 30*time.Second, 1)

	tb := ratelimiter.NewTokenBucket()

	apiKey := auth.NewAPIKey()
	jwtAuth := auth.NewJWT()

	m := metrics.NewCollector()

	proxyH := handler.NewProxy(gatewaySvc, lb, breaker, tb, apiKey, jwtAuth, m)

	cleanup := func() {
		_ = db.Close()
	}

	// Start periodic rate limiter cleanup.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			tb.Cleanup(10 * time.Minute)
		}
	}()

	return &App{
		Config:       cfg,
		AdminHandler: adminH,
		ProxyHandler: proxyH,
		Metrics:      m,
		Repo:         repo,
	}, cleanup, nil
}

// runMigrations creates the schema if it doesn't exist.
func runMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS routes (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		path            TEXT NOT NULL,
		methods         TEXT NOT NULL DEFAULT '["GET"]',
		rewrite_pattern TEXT NOT NULL DEFAULT '',
		headers         TEXT NOT NULL DEFAULT '{}',
		timeout_ns      INTEGER NOT NULL DEFAULT 30000000000,
		enabled         INTEGER NOT NULL DEFAULT 1,
		created_at      INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at      INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);

	CREATE TABLE IF NOT EXISTS backends (
		id          TEXT PRIMARY KEY,
		route_id    TEXT NOT NULL,
		url         TEXT NOT NULL,
		weight      INTEGER NOT NULL DEFAULT 1,
		health_path TEXT NOT NULL DEFAULT '',
		enabled     INTEGER NOT NULL DEFAULT 1,
		FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS auth_configs (
		id       TEXT PRIMARY KEY,
		route_id TEXT NOT NULL UNIQUE,
		type     TEXT NOT NULL DEFAULT 'api_key',
		issuer   TEXT NOT NULL DEFAULT '',
		secret   TEXT NOT NULL DEFAULT '',
		api_key  TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS rate_limits (
		id         TEXT PRIMARY KEY,
		route_id   TEXT NOT NULL UNIQUE,
		requests   INTEGER NOT NULL DEFAULT 100,
		window_ns  INTEGER NOT NULL DEFAULT 60000000000,
		per_client INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_backends_route ON backends (route_id);
	CREATE INDEX IF NOT EXISTS idx_auth_configs_route ON auth_configs (route_id);
	CREATE INDEX IF NOT EXISTS idx_rate_limits_route ON rate_limits (route_id);
	`

	_, err := db.Exec(schema)
	return err
}
