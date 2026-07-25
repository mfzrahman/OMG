package auth

import (
	"net/http"

	"github.com/omg/omg/internal/model"
)

// Authenticator validates an incoming request against an auth config.
type Authenticator interface {
	// Authenticate checks the request. Returns the identity (user ID or
	// key name) and true if authenticated, or empty string and false.
	Authenticate(r *http.Request, cfg *model.AuthConfig) (identity string, ok bool)
}
