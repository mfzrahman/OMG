package handler

import (
	"encoding/json"
	"net/http"
)

// Health returns a liveness check. Suitable for Kubernetes probes.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
