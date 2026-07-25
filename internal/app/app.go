package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omg/omg/internal/config"
	"github.com/omg/omg/internal/handler"
	"github.com/omg/omg/internal/repository"
	"github.com/omg/omg/internal/service"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver
)

// App is the dependency container. It holds every wired component so that
// handlers, services, and repositories are assembled in exactly one place.
type App struct {
	Config          *config.Config
	ProviderHandler *handler.ProviderHandler
}

// New wires the full application and returns it together with a cleanup
// function that must be deferred by the caller.
func New(cfg *config.Config) (*App, func(), error) {
	// Ensure the parent directory exists (relevant for Docker /data mount).
	if dir := filepath.Dir(cfg.DatabasePath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("db open: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, nil, fmt.Errorf("enable WAL: %w", err)
	}

	// SQLite only supports a single writer; limit to one open connection
	// to avoid "database is locked" errors under concurrent writes.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, nil, fmt.Errorf("db ping: %w", err)
	}

	// --- Wire dependencies ---
	providerRepo := repository.NewProviderSQLite(db)
	providerSvc  := service.NewProvider(providerRepo)
	providerH    := handler.NewProvider(providerSvc)

	cleanup := func() {
		if err := db.Close(); err != nil {
			// Log but don't panic during cleanup.
			_ = err
		}
	}

	return &App{
		Config:          cfg,
		ProviderHandler: providerH,
	}, cleanup, nil
}
