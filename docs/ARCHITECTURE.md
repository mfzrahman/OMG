# OMG API Gateway — Architecture & Developer Guide

**OMG (Open Model Gate)** is a high-performance, production-grade API Gateway written in Go. It provides routing, load balancing, rate limiting, circuit breaking, authentication, and observability — all configurable at runtime through a REST admin API.

---

## Project Structure

```
omg/
├── cmd/server/main.go                     # Entry point, graceful shutdown
├── config.yaml                            # Default gateway config
├── internal/
│   ├── app/app.go                         # Dependency injection container
│   ├── auth/
│   │   ├── authenticator.go               # Authenticator interface
│   │   ├── apikey.go                      # X-API-Key header auth
│   │   └── jwt.go                         # HS256 JWT auth (stdlib only)
│   ├── circuitbreaker/
│   │   └── circuitbreaker.go              # Open/half-open/closed state machine
│   ├── config/config.go                   # Environment-based configuration
│   ├── handler/
│   │   ├── admin.go                       # Admin CRUD endpoints (15 routes)
│   │   ├── health.go                      # GET /health liveness probe
│   │   ├── middleware.go                  # Recovery, RequestID, Logger, CORS
│   │   └── proxy.go                       # Full gateway pipeline orchestrator
│   ├── loadbalancer/
│   │   ├── loadbalancer.go                # Balancer interface
│   │   ├── roundrobin.go                  # Weighted round-robin
│   │   └── leastconn.go                   # Least-connections
│   ├── metrics/
│   │   └── metrics.go                     # Prometheus text format exporter
│   ├── model/
│   │   ├── route.go                       # Route, Backend, BackendWithState
│   │   ├── auth.go                        # AuthConfig
│   │   └── ratelimit.go                   # RateLimit
│   ├── ratelimiter/
│   │   └── tokenbucket.go                 # Lazy-refill token bucket
│   ├── repository/
│   │   ├── route.go                       # RouteRepository interface
│   │   └── route_sqlite.go                # SQLite CRUD (15 methods)
│   ├── router/router.go                   # ServeMux wiring + middleware stack
│   └── service/
│       ├── admin.go                       # Admin business logic + validation
│       └── gateway.go                     # Route matching + path rewriting
├── pkg/
│   ├── response/json.go                   # {data, error} JSON envelope
│   └── requestid/requestid.go            # Request ID generation/extraction
├── migrations/
│   ├── 001_create_tables.up.sql
│   └── 001_create_tables.down.sql
├── Dockerfile                             # Multi-stage, FROM scratch (~8 MB)
├── compose.yaml
├── Makefile
├── go.mod / go.sum
└── .gitignore
```

## Architecture

### Layered Design

```
handler  →  service  →  repository (interface)
   ↓           ↓            ↓
  HTTP     Business       SQLite
  parsing   logic          impl
```

Dependencies point **inward**. The repository is consumed as an interface, so the database can be swapped without touching business logic or HTTP code.

### Request Pipeline

```
Client → Recovery → RequestID → Logger → CORS → ServeMux
  ├─ GET  /health          → Health handler
  ├─ GET  /metrics         → Prometheus metrics
  ├─ *    /admin/*         → Admin CRUD handler
  └─ *    /*               → Proxy handler
       ├─ 1. Match route (method + path + headers)
       ├─ 2. Authenticate (JWT or API key, if configured)
       ├─ 3. Rate limit (token bucket, per-route or per-client)
       ├─ 4. Load balance (weighted round-robin or least-conn)
       ├─ 5. Circuit breaker check (fail fast if open)
       ├─ 6. Rewrite path (parameter substitution)
       ├─ 7. Reverse proxy (httputil.ReverseProxy)
       └─ 8. Record metrics + update circuit breaker state
```

### Component Details

**Circuit Breaker** — Three states:
- **Closed**: Normal operation. Failures increment a counter.
- **Open**: After N consecutive failures (default 5), all requests fail fast with 503.
- **Half-Open**: After cooldown (default 30s), a single probe is allowed. Success resets to Closed; failure returns to Open.

**Rate Limiter** — Lazy-refill token bucket:
- No background goroutines. Refill computed on each `Allow()` call.
- Tokens added = `elapsed_seconds × (requests_per_window / window_seconds)`.
- Keys are `route_id` (route-wide) or `route_id:client_ip` (per-client).

**Load Balancer** — Two strategies:
- **RoundRobin** (default): Weighted selection using an atomic counter.
- **LeastConn**: Picks the backend with fewest active connections.

**Authentication** — Two methods, both stdlib-only:
- **API Key**: Checks `X-API-Key` header against stored key.
- **JWT HS256**: Parses token, verifies HMAC-SHA256 signature, checks expiry and issuer.

**Metrics** — Prometheus text exposition format at `/metrics`:
- `omg_route_requests_total{route="..."}`
- `omg_route_errors_total{route="..."}`
- `omg_route_latency_ms{route="..."}`
- `omg_route_active_connections{route="..."}`
- `omg_backend_requests_total{backend="..."}`
- `omg_backend_errors_total{backend="..."}`

## API Reference

### Admin API (`/admin`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/admin/routes` | List all routes |
| `POST` | `/admin/routes` | Create a route |
| `GET` | `/admin/routes/{id}` | Get route by ID |
| `PUT` | `/admin/routes/{id}` | Update route |
| `DELETE` | `/admin/routes/{id}` | Delete route + associated config |
| `POST` | `/admin/routes/{id}/backends` | Add backend to route |
| `PUT` | `/admin/routes/{id}/backends/{bid}` | Update backend |
| `DELETE` | `/admin/routes/{id}/backends/{bid}` | Remove backend |
| `GET` | `/admin/routes/{id}/auth` | Get auth config |
| `PUT` | `/admin/routes/{id}/auth` | Set auth config |
| `DELETE` | `/admin/routes/{id}/auth` | Remove auth config |
| `GET` | `/admin/routes/{id}/ratelimit` | Get rate limit |
| `PUT` | `/admin/routes/{id}/ratelimit` | Set rate limit |
| `DELETE` | `/admin/routes/{id}/ratelimit` | Remove rate limit |

### Public Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Gateway liveness probe |
| `GET` | `/metrics` | Prometheus metrics |
| `*` | `/*` | Proxied through gateway pipeline |

### Response Format

All responses use a consistent JSON envelope:

```json
// Success
{ "data": { ... } }

// Error
{ "error": "message" }
```

## Configuration

All config via environment variables (12-factor app style):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_PATH` | `omg.db` | SQLite database file path |
| `READ_TIMEOUT` | `10s` | Server read timeout |
| `WRITE_TIMEOUT` | `30s` | Server write timeout |
| `IDLE_TIMEOUT` | `120s` | Keep-alive idle timeout |
| `SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown deadline |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

A `config.yaml` file is available for declarative route seeding (loaded on first startup if the routes table is empty).

## Quick Start

```bash
# Start the gateway (auto-creates omg.db)
make run

# Create a route
curl -X POST http://localhost:8080/admin/routes \
  -H "Content-Type: application/json" \
  -d '{"name":"my-api","path":"/api/users/{id}","methods":["GET"],"rewrite_pattern":"/users/{id}","enabled":true}'

# Add a backend
curl -X POST http://localhost:8080/admin/routes/{route_id}/backends \
  -H "Content-Type: application/json" \
  -d '{"url":"http://my-service:4000","weight":1}'

# Test the proxy
curl http://localhost:8080/api/users/42

# View metrics
curl http://localhost:8080/metrics
```

## Dependencies

Only 1 third-party package:
- `modernc.org/sqlite` — Pure-Go SQLite driver (no CGO required)

Everything else uses Go standard library: `net/http`, `net/http/httputil`, `crypto/hmac`, `crypto/sha256`, `encoding/json`, `log/slog`.

## Design Decisions

1. **Stdlib HTTP router** — Go 1.22+ `ServeMux` supports `METHOD /path/{param}` patterns natively. No external router dependency.
2. **Pure-Go SQLite** — `modernc.org/sqlite` requires no CGO, enabling `FROM scratch` Docker builds and cross-compilation.
3. **Lazy-refill token bucket** — No background goroutines. Tokens are refilled on each `Allow()` call.
4. **Prometheus without client library** — The exposition format is a simple text format; generating it directly avoids a large dependency.
5. **JWT without a library** — HS256 validation is implemented in ~70 lines using only `crypto/hmac` and `crypto/sha256`.
6. **Single DB connection** — SQLite serialises writes; `SetMaxOpenConns(1)` avoids `database is locked` errors. WAL mode allows concurrent reads.
7. **Ingress-based admin security** — Admin API runs on the same port under `/admin`. Secure by placing behind a firewall or adding an auth middleware.
