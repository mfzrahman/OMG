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

	// Health check — no auth required.
	mux.HandleFunc("GET /health", handler.Health)

	// Provider CRUD — /api/v1 namespace.
	mux.HandleFunc("POST   /api/v1/providers",     a.ProviderHandler.Create)
	mux.HandleFunc("GET    /api/v1/providers",     a.ProviderHandler.List)
	mux.HandleFunc("GET    /api/v1/providers/{id}", a.ProviderHandler.GetByID)
	mux.HandleFunc("DELETE /api/v1/providers/{id}", a.ProviderHandler.Delete)

	// Apply middleware stack (outermost first).
	return handler.Recovery(
		handler.Logger(
			handler.CORS(mux),
		),
	)
}
