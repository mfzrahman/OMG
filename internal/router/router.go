package router

import (
	"net/http"

	"github.com/omg/omg/internal/app"
	"github.com/omg/omg/internal/handler"
)

// New builds the HTTP handler tree with all routes and middleware.
// Uses Go 1.22+ enhanced ServeMux patterns (METHOD /path).
func New(a *app.App) http.Handler {
	mux := http.NewServeMux()

	// Health check — public, no auth.
	mux.HandleFunc("GET /health", handler.Health)

	// Metrics — public Prometheus endpoint.
	mux.HandleFunc("GET /metrics", a.Metrics.ServeHTTP)

	// Admin API — CRUD for routes, backends, auth, rate limits.
	a.AdminHandler.RegisterRoutes(mux)

	// Catch-all — all unmatched requests go through the gateway proxy
	// pipeline. Use the root pattern to catch everything.
	mux.HandleFunc("/", a.ProxyHandler.ServeHTTP)

	// Apply middleware (outermost first).
	return handler.Recovery(
		handler.RequestID(
			handler.Logger(
				handler.CORS(mux),
			),
		),
	)
}
