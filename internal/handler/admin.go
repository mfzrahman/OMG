package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/omg/omg/internal/model"
	"github.com/omg/omg/internal/service"
	"github.com/omg/omg/pkg/response"
)

// AdminHandler exposes the configuration API for managing routes,
// backends, auth, and rate limits at runtime.
type AdminHandler struct {
	svc *service.AdminService
}

// NewAdmin creates a new AdminHandler.
func NewAdmin(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// RegisterRoutes registers all admin endpoints on the provided mux.
func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET    /admin/routes", h.ListRoutes)
	mux.HandleFunc("POST   /admin/routes", h.CreateRoute)
	mux.HandleFunc("GET    /admin/routes/{id}", h.GetRoute)
	mux.HandleFunc("PUT    /admin/routes/{id}", h.UpdateRoute)
	mux.HandleFunc("DELETE /admin/routes/{id}", h.DeleteRoute)

	mux.HandleFunc("POST   /admin/routes/{id}/backends", h.AddBackend)
	mux.HandleFunc("PUT    /admin/routes/{id}/backends/{bid}", h.UpdateBackend)
	mux.HandleFunc("DELETE /admin/routes/{id}/backends/{bid}", h.RemoveBackend)

	mux.HandleFunc("GET    /admin/routes/{id}/auth", h.GetAuth)
	mux.HandleFunc("PUT    /admin/routes/{id}/auth", h.SetAuth)
	mux.HandleFunc("DELETE /admin/routes/{id}/auth", h.RemoveAuth)

	mux.HandleFunc("GET    /admin/routes/{id}/ratelimit", h.GetRateLimit)
	mux.HandleFunc("PUT    /admin/routes/{id}/ratelimit", h.SetRateLimit)
	mux.HandleFunc("DELETE /admin/routes/{id}/ratelimit", h.RemoveRateLimit)
}

// ---- Routes ----

func (h *AdminHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.svc.ListRoutes(r.Context())
	if err != nil {
		slog.Error("list routes", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if routes == nil {
		routes = []model.Route{}
	}
	response.JSON(w, http.StatusOK, routes)
}

func (h *AdminHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var rt model.Route
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Parse timeout from string if provided
	if rt.Timeout == 0 {
		rt.Timeout = 30 * time.Second
	}
	if err := h.svc.CreateRoute(r.Context(), &rt); err != nil {
		slog.Error("create route", "error", err)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, rt)
}

func (h *AdminHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	rt, err := h.svc.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusNotFound, "route not found")
		return
	}
	response.JSON(w, http.StatusOK, rt)
}

func (h *AdminHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	var rt model.Route
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rt.ID = r.PathValue("id")
	if err := h.svc.UpdateRoute(r.Context(), &rt); err != nil {
		slog.Error("update route", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, rt)
}

func (h *AdminHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteRoute(r.Context(), r.PathValue("id")); err != nil {
		slog.Error("delete route", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Backends ----

func (h *AdminHandler) AddBackend(w http.ResponseWriter, r *http.Request) {
	var b model.Backend
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	b.RouteID = r.PathValue("id")
	if b.Weight <= 0 {
		b.Weight = 1
	}
	b.Enabled = true // default to enabled
	if err := h.svc.AddBackend(r.Context(), &b); err != nil {
		slog.Error("add backend", "error", err)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, b)
}

func (h *AdminHandler) UpdateBackend(w http.ResponseWriter, r *http.Request) {
	var b model.Backend
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	b.ID = r.PathValue("bid")
	if err := h.svc.UpdateBackend(r.Context(), &b); err != nil {
		slog.Error("update backend", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, b)
}

func (h *AdminHandler) RemoveBackend(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveBackend(r.Context(), r.PathValue("bid")); err != nil {
		slog.Error("remove backend", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Auth ----

func (h *AdminHandler) GetAuth(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.GetAuthConfig(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusNotFound, "auth config not found")
		return
	}
	response.JSON(w, http.StatusOK, cfg)
}

func (h *AdminHandler) SetAuth(w http.ResponseWriter, r *http.Request) {
	var cfg model.AuthConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg.RouteID = r.PathValue("id")
	if err := h.svc.SetAuthConfig(r.Context(), &cfg); err != nil {
		slog.Error("set auth", "error", err)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, cfg)
}

func (h *AdminHandler) RemoveAuth(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveAuthConfig(r.Context(), r.PathValue("id")); err != nil {
		slog.Error("remove auth", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Rate Limits ----

func (h *AdminHandler) GetRateLimit(w http.ResponseWriter, r *http.Request) {
	rl, err := h.svc.GetRateLimit(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusNotFound, "rate limit not found")
		return
	}
	response.JSON(w, http.StatusOK, rl)
}

func (h *AdminHandler) SetRateLimit(w http.ResponseWriter, r *http.Request) {
	var rl model.RateLimit
	if err := json.NewDecoder(r.Body).Decode(&rl); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rl.RouteID = r.PathValue("id")
	if err := h.svc.SetRateLimit(r.Context(), &rl); err != nil {
		slog.Error("set rate limit", "error", err)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, rl)
}

func (h *AdminHandler) RemoveRateLimit(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveRateLimit(r.Context(), r.PathValue("id")); err != nil {
		slog.Error("remove rate limit", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
