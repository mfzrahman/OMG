package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/omg/omg/internal/model"
)

// JWTAuthenticator validates HS256-signed JWTs from the Authorization
// header. Uses only stdlib crypto — no third-party JWT library.
type JWTAuthenticator struct{}

// NewJWT creates a new JWT authenticator.
func NewJWT() *JWTAuthenticator {
	return &JWTAuthenticator{}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Iss string `json:"iss"`
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
}

func (a *JWTAuthenticator) Authenticate(r *http.Request, cfg *model.AuthConfig) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(auth, "Bearer ")

	sub, err := verifyHS256(token, cfg.Secret, cfg.Issuer)
	if err != nil {
		return "", false
	}
	return sub, true
}

// verifyHS256 validates a JWT token signed with HMAC-SHA256.
func verifyHS256(token, secret, issuer string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	// Decode header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "HS256" {
		return "", fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(sig, expectedSig) {
		return "", fmt.Errorf("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}
	var payload jwtPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", fmt.Errorf("parse payload: %w", err)
	}

	// Validate expiry
	if payload.Exp != 0 && time.Now().Unix() > payload.Exp {
		return "", fmt.Errorf("token expired")
	}

	// Validate issuer if configured
	if issuer != "" && payload.Iss != issuer {
		return "", fmt.Errorf("invalid issuer: %s", payload.Iss)
	}

	return payload.Sub, nil
}
