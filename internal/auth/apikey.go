package auth

import (
	"net/http"

	"github.com/omg/omg/internal/model"
)

// APIKeyAuthenticator checks the X-API-Key header.
type APIKeyAuthenticator struct{}

// NewAPIKey creates a new API key authenticator.
func NewAPIKey() *APIKeyAuthenticator {
	return &APIKeyAuthenticator{}
}

func (a *APIKeyAuthenticator) Authenticate(r *http.Request, cfg *model.AuthConfig) (string, bool) {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return "", false
	}
	if key != cfg.APIKey {
		return "", false
	}
	return "api_key:" + mask(key), true
}

func mask(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****"
}
