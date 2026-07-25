package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/omg/omg/internal/app"
	"github.com/omg/omg/internal/config"
	"github.com/omg/omg/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting OMG — Open Model Gate", "port", cfg.Port)

	a, cleanup, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialise application", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router.New(a),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server in a goroutine so we can listen for shutdown signals.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Block until a signal is received.
	<-quit
	slog.Info("shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
