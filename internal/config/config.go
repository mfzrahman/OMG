package config

import (
	"os"
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	Port        string
	DatabasePath string
	// Read timeouts for the HTTP server.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	// ShutdownTimeout is the deadline for graceful shutdown.
	ShutdownTimeout time.Duration
	// LogLevel controls slog verbosity ("debug", "info", "warn", "error").
	LogLevel string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:            envOrDefault("PORT", "8080"),
		DatabasePath:    envOrDefault("DATABASE_PATH", "omg.db"),
		ReadTimeout:     parseDurationEnv("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    parseDurationEnv("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:     parseDurationEnv("IDLE_TIMEOUT", 120*time.Second),
		ShutdownTimeout: parseDurationEnv("SHUTDOWN_TIMEOUT", 15*time.Second),
		LogLevel:        envOrDefault("LOG_LEVEL", "info"),
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return defaultVal
		}
		return d
	}
	return defaultVal
}
