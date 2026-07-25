package model

// AuthConfig defines authentication requirements for a route.
type AuthConfig struct {
	ID      string `json:"id"`
	RouteID string `json:"route_id"`
	Type    string `json:"type"` // "jwt" or "api_key"
	Issuer  string `json:"issuer,omitempty"`
	Secret  string `json:"-"` // never serialised to JSON
	APIKey  string `json:"-"` // never serialised to JSON
}
