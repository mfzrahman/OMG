package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/omg/omg/internal/model"
	"github.com/omg/omg/internal/service"
	"github.com/omg/omg/pkg/response"
)

// ProviderHandler handles HTTP requests for AI model providers.
type ProviderHandler struct {
	svc *service.ProviderService
}

// NewProvider creates a new ProviderHandler.
func NewProvider(svc *service.ProviderService) *ProviderHandler {
	return &ProviderHandler{svc: svc}
}

// Create registers a new AI model provider.
func (h *ProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	provider, err := h.svc.Register(r.Context(), req)
	if err != nil {
		slog.Error("failed to register provider", "error", err)
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, provider)
}

// GetByID returns a single provider.
func (h *ProviderHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	provider, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("failed to get provider", "id", id, "error", err)
		response.Error(w, http.StatusNotFound, "provider not found")
		return
	}
	response.JSON(w, http.StatusOK, provider)
}

// List returns all registered providers.
func (h *ProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	providers, err := h.svc.List(r.Context())
	if err != nil {
		slog.Error("failed to list providers", "error", err)
		response.Error(w, http.StatusInternalServerError, "failed to list providers")
		return
	}
	response.JSON(w, http.StatusOK, providers)
}

// Delete removes a provider.
func (h *ProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Remove(r.Context(), id); err != nil {
		slog.Error("failed to delete provider", "id", id, "error", err)
		response.Error(w, http.StatusInternalServerError, "failed to delete provider")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
