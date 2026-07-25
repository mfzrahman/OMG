package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/omg/omg/internal/model"
	"github.com/omg/omg/internal/repository"
)

// GatewayService handles the core gateway logic: route matching and
// pipeline orchestration.
type GatewayService struct {
	repo repository.RouteRepository
}

// NewGateway creates a new GatewayService.
func NewGateway(repo repository.RouteRepository) *GatewayService {
	return &GatewayService{repo: repo}
}

// MatchResult holds a matched route and its associated configuration.
type MatchResult struct {
	Route      *model.Route
	Backends   []model.Backend
	AuthConfig *model.AuthConfig
	RateLimit  *model.RateLimit
}

// Match finds a route matching the request by method, path, and
// optional headers. Returns nil if no route matches.
func (s *GatewayService) Match(ctx context.Context, r *http.Request) (*MatchResult, error) {
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("match: list routes: %w", err)
	}

	for i := range routes {
		rt := &routes[i]

		if !rt.Enabled {
			continue
		}

		// Method match
		if !matchMethod(rt.Methods, r.Method) {
			continue
		}

		// Path match with parameter extraction
		params, ok := matchPath(rt.Path, r.URL.Path)
		if !ok {
			continue
		}

		// Header match
		if !matchHeaders(rt.Headers, r.Header) {
			continue
		}

		// Found a match — load associated config.
		backends, err := s.repo.ListBackends(ctx, rt.ID)
		if err != nil {
			return nil, fmt.Errorf("match: list backends: %w", err)
		}
		if len(backends) == 0 {
			return nil, fmt.Errorf("match: route %s has no backends", rt.ID)
		}

		authCfg, _ := s.repo.GetAuthConfig(ctx, rt.ID)
		rateLimit, _ := s.repo.GetRateLimit(ctx, rt.ID)

		_ = params // path params stored for rewrite

		return &MatchResult{
			Route:      rt,
			Backends:   backends,
			AuthConfig: authCfg,
			RateLimit:  rateLimit,
		}, nil
	}

	return nil, nil
}

// PathParams extracts path parameters from a matched route.
func PathParams(routePath, requestPath string) map[string]string {
	routeParts := strings.Split(strings.Trim(routePath, "/"), "/")
	reqParts := strings.Split(strings.Trim(requestPath, "/"), "/")

	params := make(map[string]string)
	for i, part := range routeParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := part[1 : len(part)-1]
			if i < len(reqParts) {
				params[name] = reqParts[i]
			}
		}
	}
	return params
}

// RewritePath applies the rewrite pattern using path parameters.
func RewritePath(pattern string, params map[string]string) string {
	result := pattern
	for k, v := range params {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

func matchMethod(allowed []string, method string) bool {
	for _, m := range allowed {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func matchPath(pattern, path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, pp := range patternParts {
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			params[pp[1:len(pp)-1]] = pathParts[i]
		} else if pp != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

func matchHeaders(required map[string]string, headers http.Header) bool {
	for k, v := range required {
		if headers.Get(k) != v {
			return false
		}
	}
	return true
}
