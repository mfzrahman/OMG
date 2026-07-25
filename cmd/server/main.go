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

	level := new(slog.LevelVar)
	switch cfg.LogLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	slog.Info("starting OMG API Gateway", "port", cfg.Port)

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

	// Start server in a goroutine for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("gateway ready", "port", cfg.Port, "health", "http://localhost:"+cfg.Port+"/health")

	// Block until signal.
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("gateway stopped")
}
