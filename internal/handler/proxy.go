package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/omg/omg/internal/auth"
	"github.com/omg/omg/internal/circuitbreaker"
	"github.com/omg/omg/internal/loadbalancer"
	"github.com/omg/omg/internal/metrics"
	"github.com/omg/omg/internal/model"
	"github.com/omg/omg/internal/ratelimiter"
	"github.com/omg/omg/internal/service"
	"github.com/omg/omg/pkg/requestid"
	"github.com/omg/omg/pkg/response"
)

// ProxyHandler is the central handler for proxied requests. It
// orchestrates the full gateway pipeline.
type ProxyHandler struct {
	gateway    *service.GatewayService
	lb         loadbalancer.Balancer
	breaker    *circuitbreaker.CircuitBreaker
	rateLimiter *ratelimiter.TokenBucket
	apiKeyAuth *auth.APIKeyAuthenticator
	jwtAuth    *auth.JWTAuthenticator
	metrics    *metrics.Collector

	// Transport for the reverse proxy.
	transport *http.Transport
}

// NewProxy creates a new ProxyHandler.
func NewProxy(
	gateway *service.GatewayService,
	lb loadbalancer.Balancer,
	breaker *circuitbreaker.CircuitBreaker,
	rl *ratelimiter.TokenBucket,
	apikey *auth.APIKeyAuthenticator,
	jwt *auth.JWTAuthenticator,
	m *metrics.Collector,
) *ProxyHandler {
	return &ProxyHandler{
		gateway:     gateway,
		lb:          lb,
		breaker:     breaker,
		rateLimiter: rl,
		apiKeyAuth:  apikey,
		jwtAuth:     jwt,
		metrics:     m,
		transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
		},
	}
}

// ServeHTTP handles every unmatched request through the gateway
// pipeline.
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Route matching
	match, err := h.gateway.Match(ctx, r)
	if err != nil || match == nil {
		slog.Warn("no matching route",
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", requestid.FromContext(ctx),
		)
		response.Error(w, http.StatusNotFound, "no matching route")
		return
	}

	route := match.Route
	h.metrics.IncActiveConns(route.ID)
	defer h.metrics.DecActiveConns(route.ID)

	// 2. Authentication
	if match.AuthConfig != nil {
		identity, ok := h.authenticate(r, match.AuthConfig)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "authentication failed")
			return
		}
		slog.Debug("authenticated", "route", route.Name, "identity", identity)
	}

	// 3. Rate limiting
	if match.RateLimit != nil {
		key := limitKey(r, match)
		allowed, _ := h.rateLimiter.Allow(ctx, key, match.RateLimit)
		if !allowed {
			response.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
	}

	// 4. Build backend state list
	backends := make([]*model.BackendWithState, len(match.Backends))
	for i := range match.Backends {
		b := &match.Backends[i]
		backends[i] = &model.BackendWithState{
			Backend:      *b,
			ActiveConns:  0,
			CircuitState: h.breaker.State(b.ID),
		}
	}

	// 5. Load balancer — select a backend
	backend, err := h.lb.Select(backends)
	if err != nil {
		slog.Error("no backend available", "route", route.Name)
		response.Error(w, http.StatusBadGateway, "no backend available")
		return
	}

	// 6. Circuit breaker
	if !h.breaker.Allow(backend.ID) {
		slog.Warn("circuit open", "backend", backend.URL)
		response.Error(w, http.StatusServiceUnavailable, "backend unavailable — circuit open")
		return
	}

	// 7. Path rewriting
	params := service.PathParams(route.Path, r.URL.Path)
	targetPath := r.URL.Path
	if route.RewritePattern != "" {
		targetPath = service.RewritePath(route.RewritePattern, params)
	}

	// 8. Build reverse proxy
	target, err := url.Parse(backend.URL)
	if err != nil {
		slog.Error("invalid backend url", "url", backend.URL)
		response.Error(w, http.StatusInternalServerError, "invalid backend configuration")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = h.transport

	// Override the director to set the rewritten path and headers.
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.URL.Path = targetPath
		req.URL.RawPath = ""
		req.Host = target.Host
		// Forward gateway-specific headers.
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", r.URL.Scheme)
		req.Header.Set("X-Request-Id", requestid.FromContext(ctx))
	}

	// WebSocket detection — pass through without modification.
	if isWebSocket(r) {
		h.proxyWebSocket(w, r, target)
		return
	}

	// 9. Set per-route timeout
	if route.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, route.Timeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	// 10. Execute proxy
	start := time.Now()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	proxy.ServeHTTP(sw, r)
	latency := time.Since(start)

	// 11. Record metrics + update circuit breaker
	h.metrics.RecordRequest(route.ID, backend.ID, sw.status, latency)

	if sw.status >= 500 {
		h.breaker.Failure(backend.ID)
	} else {
		h.breaker.Success(backend.ID)
	}
}

func (h *ProxyHandler) authenticate(r *http.Request, cfg *model.AuthConfig) (string, bool) {
	switch cfg.Type {
	case "api_key":
		return h.apiKeyAuth.Authenticate(r, cfg)
	case "jwt":
		return h.jwtAuth.Authenticate(r, cfg)
	default:
		return "", false
	}
}

func isWebSocket(r *http.Request) bool {
	return r.Header.Get("Upgrade") == "websocket"
}

func (h *ProxyHandler) proxyWebSocket(w http.ResponseWriter, r *http.Request, target *url.URL) {
	// For now, pass through using the standard reverse proxy which
	// handles WebSocket upgrades transparently via http.Transport.
	// httputil.ReverseProxy already supports WebSocket.
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = h.transport
	proxy.ServeHTTP(w, r)
}

func limitKey(r *http.Request, match *service.MatchResult) string {
	if match.RateLimit.PerClient {
		// Key by IP or forwarded-for header.
		client := r.Header.Get("X-Forwarded-For")
		if client == "" {
			client = r.RemoteAddr
		}
		return fmt.Sprintf("%s:%s", match.Route.ID, client)
	}
	return match.Route.ID
}
