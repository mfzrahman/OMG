# OMG — Architecture & Developer Guide

**OMG (Open Model Gate)** is an AI model API interface translator, load balancer, and proxy server. It provides a unified API gateway that lets LLM-powered tools, coding agents, and CLI utilities work with different models and providers from a single endpoint.

---

## Table of Contents

1. [Project Structure](#project-structure)
2. [Architecture Overview](#architecture-overview)
3. [Layer-by-Layer Walkthrough](#layer-by-layer-walkthrough)
4. [Configuration](#configuration)
5. [API Reference](#api-reference)
6. [Database](#database)
7. [Development Workflow](#development-workflow)
8. [Deployment](#deployment)
9. [Design Decisions](#design-decisions)

---

## Project Structure

```
omg/
├── cmd/
│   └── server/
│       └── main.go                  # Application entry point
│
├── internal/
│   ├── app/
│   │   └── app.go                   # Dependency injection container
│   ├── config/
│   │   └── config.go                # Environment-based configuration
│   ├── handler/
│   │   ├── health.go                # Liveness/readiness probe
│   │   ├── middleware.go            # Recovery, logging, CORS
│   │   └── provider.go              # Provider CRUD HTTP handlers
│   ├── model/
│   │   └── provider.go              # Domain types and DTOs
│   ├── repository/
│   │   ├── provider.go              # Data-access interface
│   │   └── provider_sqlite.go       # SQLite implementation
│   ├── router/
│   │   └── router.go                # Route registration and middleware chain
│   └── service/
│       └── provider.go              # Business logic layer
│
├── migrations/
│   ├── 001_create_providers.up.sql  # Forward migration
│   └── 001_create_providers.down.sql # Rollback migration
│
├── pkg/
│   └── response/
│       └── json.go                  # Shared JSON response helpers
│
├── docs/
│   └── ARCHITECTURE.md              # This document
│
├── .env.example                     # Configuration template
├── compose.yaml                     # Docker Compose for local development
├── Dockerfile                       # Multi-stage production build
├── Makefile                         # Task runner
├── go.mod                           # Go module definition
├── go.sum                           # Dependency checksums
└── README.md                        # Project overview
```

### Directory Conventions

| Directory | Purpose | Visibility |
|-----------|---------|------------|
| `cmd/` | One subdirectory per executable. Each contains a thin `main.go` that wires the app and starts it. | Entry point |
| `internal/` | Application code that must **not** be imported by external modules. The Go compiler enforces this. | Private |
| `pkg/` | Shared utilities that are safe for external consumption. | Public |
| `migrations/` | Versioned SQL migration files (`NNN_description.{up,down}.sql`). | Infrastructure |
| `docs/` | Project documentation. | Reference |

---

## Architecture Overview

The project follows a **layered architecture** with strict dependency direction:

```
┌──────────────────────────────────────────────┐
│                   cmd/server                  │  Entry point
│               (parse config, start)            │
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│                  router                       │  HTTP routing
│          (method + path patterns)              │
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│                middleware                     │  Cross-cutting
│     Recovery → Logger → CORS                  │  concerns
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│                 handler                       │  HTTP concern
│   Parse request → call service → write response│
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│                 service                       │  Business logic
│       Validation, orchestration, rules        │
└──────────────────┬───────────────────────────┘
                   │  (depends on interface)
┌──────────────────▼───────────────────────────┐
│               repository                      │  Data access
│          Interface + SQLite impl               │
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│                 model                         │  Data structures
│           (no dependencies)                    │
└──────────────────────────────────────────────┘
```

### Dependency Rule

**Dependencies point inward.** Each layer only depends on the layer directly below it:

- `handler` → depends on `service`
- `service` → depends on `repository` (the **interface**, never the implementation)
- `repository` → depends on `model` and the database driver
- `model` → depends on **nothing**

This means you can swap the database (SQLite → Postgres → in-memory) without touching a single line of business logic or HTTP code.

### The `App` Struct — Single Dependency Container

All wired components live in one struct ([internal/app/app.go](internal/app/app.go)):

```go
type App struct {
    Config          *config.Config
    ProviderHandler *handler.ProviderHandler
}
```

There is **no global state**. Every handler, service, and repository receives its dependencies through constructor injection. The `New()` function is the single place where everything is assembled:

```
config → db → repository → service → handler → router → http.Server
```

---

## Layer-by-Layer Walkthrough

### 1. `cmd/server/main.go` — Entry Point

**File:** [cmd/server/main.go](cmd/server/main.go)

Responsibilities:
- Load configuration via `config.Load()`
- Initialise the application via `app.New(cfg)`
- Create an `http.Server` with sensible timeouts
- Listen for `SIGINT`/`SIGTERM` and perform **graceful shutdown**

**Graceful shutdown flow:**
1. Signal received → stop accepting new connections
2. Give in-flight requests up to `SHUTDOWN_TIMEOUT` (default 15s) to complete
3. Close the database connection pool
4. Exit

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
// ... server runs in goroutine ...
<-quit
ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
defer cancel()
srv.Shutdown(ctx)
```

### 2. `internal/config/config.go` — Configuration

**File:** [internal/config/config.go](internal/config/config.go)

All configuration comes from **environment variables**. There are no config files to parse at runtime.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_PATH` | `omg.db` | Path to the SQLite database file |
| `READ_TIMEOUT` | `10s` | Maximum duration for reading the entire request |
| `WRITE_TIMEOUT` | `30s` | Maximum duration before timing out writes of the response |
| `IDLE_TIMEOUT` | `120s` | Maximum amount of time to wait for the next request when keep-alives are enabled |
| `SHUTDOWN_TIMEOUT` | `15s` | Deadline for graceful shutdown |
| `LOG_LEVEL` | `info` | Slog verbosity: `debug`, `info`, `warn`, `error` |

**Why environment variables?**
- Works everywhere without file I/O (containers, PaaS, systemd)
- No parsing logic to maintain
- Standard pattern in the Go ecosystem
- Easy to override per deployment

### 3. `internal/app/app.go` — Dependency Injection

**File:** [internal/app/app.go](internal/app/app.go)

This is the **composition root** — the single place where all concrete types are wired together.

**Construction sequence:**
1. Create parent directory for the database file if needed (important for Docker volume mounts like `/data/`)
2. Open SQLite connection via `database/sql`
3. Enable **WAL journal mode** for better concurrent read performance
4. Set `MaxOpenConns(1)` — SQLite serialises all writes; a single connection avoids `database is locked` errors
5. Ping the database to verify connectivity
6. Wire: `db → repository → service → handler`
7. Return the `App` struct and a `cleanup` function

```go
// WAL mode means readers don't block writers and vice versa.
db.Exec("PRAGMA journal_mode=WAL")

// Single writer constraint.
db.SetMaxOpenConns(1)
```

### 4. `internal/model/provider.go` — Domain Types

**File:** [internal/model/provider.go](internal/model/provider.go)

Plain structs with **no behaviour** and **no dependencies**. This package is importable by every other layer.

**Key types:**

```go
// Database row
type Provider struct {
    ID        string
    Name      string
    BaseURL   string
    APIKey    string    // json:"-" — never serialised
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Request DTO (Data Transfer Object)
type CreateProviderRequest struct {
    Name    string `json:"name"`
    BaseURL string `json:"base_url"`
    APIKey  string `json:"api_key"`
}

// Response DTO — omits the secret key
type CreateProviderResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    BaseURL   string    `json:"base_url"`
    CreatedAt time.Time `json:"created_at"`
}
```

**Security note:** `Provider.APIKey` has the struct tag `json:"-"`. It is **never** serialised to JSON, even if accidentally passed to `json.Marshal`. The response DTO `CreateProviderResponse` also excludes the key field entirely.

### 5. `internal/repository/` — Data Access

**Files:**
- [internal/repository/provider.go](internal/repository/provider.go) — interface
- [internal/repository/provider_sqlite.go](internal/repository/provider_sqlite.go) — SQLite implementation

The interface defines **what** data operations exist:

```go
type ProviderRepository interface {
    Create(ctx context.Context, p *model.Provider) error
    GetByID(ctx context.Context, id string) (*model.Provider, error)
    List(ctx context.Context) ([]model.Provider, error)
    Delete(ctx context.Context, id string) error
}
```

The SQLite implementation uses `?` placeholders (SQLite syntax) and wraps errors with context:

```go
if err != nil {
    return fmt.Errorf("insert provider: %w", err)
}
```

**Every method accepts a `context.Context`** as its first argument. This is the Go convention for deadline propagation, cancellation, and request-scoped values.

**Adding a new database backend** (e.g., Postgres):
1. Create `provider_pg.go` implementing the same interface
2. Change one line in `app.go`: `NewProviderSQLite(db)` → `NewProviderPG(db)`
3. Nothing else changes

### 6. `internal/service/provider.go` — Business Logic

**File:** [internal/service/provider.go](internal/service/provider.go)

This is where **all business rules** live. The handler never contains logic beyond request parsing; the repository never contains logic beyond data access.

**Validation examples:**
```go
if req.Name == "" {
    return nil, fmt.Errorf("provider name is required")
}
if req.BaseURL == "" {
    return nil, fmt.Errorf("provider base_url is required")
}
```

The service:
- Validates input
- Generates IDs (placeholder implementation — replace with ULID/UUID for production)
- Sets timestamps in UTC
- Delegates persistence to the repository interface

### 7. `internal/handler/` — HTTP Layer

**Files:**
- [internal/handler/health.go](internal/handler/health.go) — liveness probe
- [internal/handler/provider.go](internal/handler/provider.go) — provider CRUD
- [internal/handler/middleware.go](internal/handler/middleware.go) — cross-cutting concerns

#### Handlers

Each handler method follows the same pattern:

```go
func (h *ProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var req model.CreateProviderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        response.Error(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // 2. Call service
    provider, err := h.svc.Register(r.Context(), req)
    if err != nil {
        slog.Error("failed to register provider", "error", err)
        response.Error(w, http.StatusInternalServerError, err.Error())
        return
    }

    // 3. Write response
    response.JSON(w, http.StatusCreated, provider)
}
```

**Handlers are thin by design.** They parse HTTP and delegate to the service layer.

#### Middleware

Applied as a composed stack — outermost first:

```go
return handler.Recovery(       // 3. Catch panics → 500
    handler.Logger(             // 2. Log method, path, status, duration
        handler.CORS(mux),      // 1. Add CORS headers
    ),
)
```

| Middleware | Order | Behaviour |
|------------|-------|-----------|
| CORS | Innermost | Adds `Access-Control-*` headers; short-circuits OPTIONS preflight |
| Logger | Middle | Logs every request with structured `slog.Info` |
| Recovery | Outermost | Recovers from panics in any downstream handler, returns 500 |

The `Logger` middleware wraps `http.ResponseWriter` with a `statusWriter` that captures the HTTP status code for logging:

```go
type statusWriter struct {
    http.ResponseWriter
    status int
}

func (sw *statusWriter) WriteHeader(code int) {
    sw.status = code
    sw.ResponseWriter.WriteHeader(code)
}
```

### 8. `internal/router/router.go` — Routing

**File:** [internal/router/router.go](internal/router/router.go)

Uses **Go 1.22+ enhanced `http.ServeMux`** patterns. No third-party router required.

```go
mux.HandleFunc("POST   /api/v1/providers",     a.ProviderHandler.Create)
mux.HandleFunc("GET    /api/v1/providers",     a.ProviderHandler.List)
mux.HandleFunc("GET    /api/v1/providers/{id}", a.ProviderHandler.GetByID)
mux.HandleFunc("DELETE /api/v1/providers/{id}", a.ProviderHandler.Delete)
```

**Pattern features used:**
- `METHOD /path` — method-based routing (no `if r.Method == "POST"` checks)
- `{id}` — path parameters, extracted via `r.PathValue("id")`
- No external router dependency

### 9. `pkg/response/json.go` — Response Helpers

**File:** [pkg/response/json.go](pkg/response/json.go)

Every API response uses a **consistent JSON envelope**:

```json
// Success
{ "data": { ... } }

// Error
{ "error": "human-readable message" }
```

This means clients always know where to look:
- Success payload → `.data`
- Error message → `.error`

The package provides two functions:

| Function | Use Case | Example Status |
|----------|----------|----------------|
| `response.JSON(w, status, data)` | Successful responses | 200, 201 |
| `response.Error(w, status, msg)` | Error responses | 400, 404, 500 |

---

## Configuration

### Full `.env` Reference

```bash
PORT=8080
DATABASE_PATH=omg.db
READ_TIMEOUT=10s
WRITE_TIMEOUT=30s
IDLE_TIMEOUT=120s
SHUTDOWN_TIMEOUT=15s
LOG_LEVEL=info
```

**Copy and customise:**
```bash
cp .env.example .env
```

### Log Levels

| Level | When to Use |
|-------|-------------|
| `debug` | Local development — verbose request/response logging |
| `info` | Production default — request logs, startup messages |
| `warn` | Recoverable issues — slow queries, retryable failures |
| `error` | Things that need attention — panics recovered, DB failures |

---

## API Reference

### Base URL

```
http://localhost:8080
```

### Endpoints

#### Health Check

```
GET /health
```

**Response 200:**
```json
{ "status": "ok" }
```

Used for Kubernetes liveness/readiness probes and load balancer health checks.

---

#### Create Provider

```
POST /api/v1/providers
```

**Request Body:**
```json
{
  "name": "OpenAI",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-..."
}
```

**Response 201:**
```json
{
  "data": {
    "id": "prv_1753456789123456789",
    "name": "OpenAI",
    "base_url": "https://api.openai.com/v1",
    "created_at": "2026-07-25T12:00:00Z"
  }
}
```

> **Note:** The `api_key` is accepted in the request but **never returned** in any response.

---

#### List Providers

```
GET /api/v1/providers
```

**Response 200:**
```json
{
  "data": [
    {
      "id": "prv_1753456789123456789",
      "name": "OpenAI",
      "base_url": "https://api.openai.com/v1",
      "created_at": "2026-07-25T12:00:00Z",
      "updated_at": "2026-07-25T12:00:00Z"
    }
  ]
}
```

---

#### Get Provider by ID

```
GET /api/v1/providers/{id}
```

**Response 200:**
```json
{
  "data": {
    "id": "prv_1753456789123456789",
    "name": "OpenAI",
    "base_url": "https://api.openai.com/v1",
    "created_at": "2026-07-25T12:00:00Z",
    "updated_at": "2026-07-25T12:00:00Z"
  }
}
```

**Response 404:**
```json
{ "error": "provider not found" }
```

---

#### Delete Provider

```
DELETE /api/v1/providers/{id}
```

**Response 204:** (no body)

---

### Error Responses

All errors follow the envelope format:

```json
{ "error": "descriptive message" }
```

| Status | Meaning |
|--------|---------|
| `400` | Invalid request body |
| `404` | Resource not found |
| `500` | Internal server error |

---

## Database

### SQLite with WAL Mode

The project uses **SQLite** via the [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) driver — a pure-Go implementation that requires **no CGO** and no system-level SQLite library.

**Why SQLite?**
- **Zero infrastructure** — no separate database server to run, manage, or secure
- **Single file** — the entire database is one portable file (`omg.db`)
- **WAL mode** — writers don't block readers, enabling concurrent read performance
- **Pure Go** — cross-compiles anywhere, works in `FROM scratch` Docker images
- **Great for single-instance services** — the gateway is the sole writer

**Connection settings:**
```go
db.Exec("PRAGMA journal_mode=WAL")  // Write-Ahead Logging
db.SetMaxOpenConns(1)               // Single writer (SQLite serialises writes)
```

### Migrations

Migrations are numbered SQL files in `migrations/`. Each migration has an `up` (forward) and `down` (rollback) file:

```
migrations/
├── 001_create_providers.up.sql
└── 001_create_providers.down.sql
```

**Running migrations:**
```bash
make migrate-up      # Apply all pending migrations
make migrate-down    # Roll back the last migration
```

Uses [`golang-migrate`](https://github.com/golang-migrate/migrate) — install with:

```bash
go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### Schema

```sql
CREATE TABLE providers (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    base_url   TEXT NOT NULL,
    api_key    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_providers_name ON providers (name);
```

---

## Development Workflow

### Prerequisites

- **Go 1.22+** (the project uses enhanced `ServeMux` patterns)
- **golang-migrate** CLI (for database migrations)
- **golangci-lint** (optional, for linting)

### Quick Start

```bash
# 1. Clone and enter the project
cd omg

# 2. Copy the environment template
cp .env.example .env

# 3. Run migrations (creates omg.db)
make migrate-up

# 4. Start the server
make run
```

The server starts on `http://localhost:8080`.

### Makefile Commands

| Command | Description |
|---------|-------------|
| `make run` | Start the server with `go run` |
| `make build` | Compile a production binary to `bin/server` |
| `make test` | Run all tests with race detection |
| `make lint` | Run `golangci-lint` across all packages |
| `make clean` | Remove build artifacts |
| `make migrate-up` | Apply all pending database migrations |
| `make migrate-down` | Roll back the most recent migration |
| `make docker-up` | Build and run with Docker Compose |
| `make docker-down` | Stop and remove containers + volumes |

### Testing

```bash
# Run all tests
make test

# Run a specific package
go test -race -v ./internal/service/...

# Run a single test
go test -race -v -run TestRegister ./internal/service/...
```

Tests use the standard library `testing` package with `httptest` for HTTP handler tests. The SQLite database can be substituted with an in-memory store for fast, isolated tests.

### Adding a New Domain Entity

Follow the layered pattern when adding a new entity (e.g., `route`, `model`, `api_key`):

1. **Model** — define structs in `internal/model/entity.go`
2. **Repository interface** — define operations in `internal/repository/entity.go`
3. **Repository implementation** — add SQLite queries in `internal/repository/entity_sqlite.go`
4. **Service** — write business logic in `internal/service/entity.go`
5. **Handler** — add HTTP handlers in `internal/handler/entity.go`
6. **Wire it** — add to `App` struct and constructor in `internal/app/app.go`
7. **Register routes** — add patterns in `internal/router/router.go`
8. **Migration** — create `migrations/002_create_entity.{up,down}.sql`

---

## Deployment

### Docker (Single Binary, Scratch Base)

**File:** [Dockerfile](Dockerfile)

The build is a **multi-stage Dockerfile**:

1. **Build stage** (`golang:1.23-alpine`) — compiles a statically-linked binary with `CGO_ENABLED=0`
2. **Runtime stage** (`scratch`) — copies only the binary and CA certificates

**Image size:** ~8 MB

```bash
# Build and tag
docker build -t omg:latest .

# Run
docker run -p 8080:8080 -v "$(pwd)/data:/data" -e DATABASE_PATH=/data/omg.db omg:latest
```

### Docker Compose

**File:** [compose.yaml](compose.yaml)

```bash
# Start
make docker-up

# Stop and clean up (including database volume)
make docker-down
```

The Compose file:
- Builds the server from the Dockerfile
- Mounts a named `data` volume at `/data` for SQLite persistence
- Sets `DATABASE_PATH=/data/omg.db`

### Production Considerations

- **TLS termination** — run behind a reverse proxy (Caddy, Nginx, Traefik) or a cloud load balancer
- **API keys** — never log them; the `Provider` struct already excludes them from JSON
- **Database backups** — copy the `omg.db` file while the server is running (WAL mode makes this safe)
- **Monitoring** — `GET /health` is your liveness endpoint; structured `slog` output can be shipped to any log aggregator
- **Single instance** — SQLite is a single-writer database; for horizontal scaling, migrate to Postgres (change one line in `app.go`)

---

## Design Decisions

### Why the Standard Library HTTP Mux?

Go 1.22 introduced method-based routing (`GET /path`) and path parameters (`{id}`) directly in `net/http`. This eliminates the need for third-party routers like `chi` or `gorilla/mux` for most use cases. Fewer dependencies means fewer things to audit, upgrade, and break.

### Why Pure-Go SQLite (`modernc.org/sqlite`)?

- **No CGO** — binaries are statically linked, cross-compilable, and deployable to `scratch`
- **No system library** — no `libsqlite3.so` dependency
- **Generated from C source** — the SQLite C code is transpiled to Go; it's the real SQLite, not a reimplementation
- **Trade-off** — ~2× slower than CGO SQLite for complex queries; irrelevant for an API gateway's configuration store

### Why `internal/` over `pkg/`?

The Go toolchain **enforces** that packages inside `internal/` cannot be imported by modules outside the current one. This prevents accidental coupling to internal implementation details. Only genuinely reusable utilities go in `pkg/`.

### Why One Connection (`SetMaxOpenConns(1)`)?

SQLite serialises all write transactions at the database level. Opening multiple connections doesn't increase write throughput — it causes `database is locked` errors when two connections try to write simultaneously. WAL mode allows concurrent reads to proceed while a write is in progress, so read-heavy workloads still perform well.

### Why Context Everywhere?

Every method that crosses a process boundary (database call, HTTP handler) accepts a `context.Context`:

```go
func (r *providerSQLite) Create(ctx context.Context, p *model.Provider) error
```

This enables:
- **Cancellation** — if the HTTP client disconnects, the database query is cancelled
- **Timeouts** — `ReadTimeout` and `WriteTimeout` propagate through the stack
- **Tracing** — request IDs and span contexts can be attached to any context

### Why the `App` Dependency Container?

Instead of package-level globals (`var db *sql.DB`) or `init()` functions, all state lives in the `App` struct. Benefits:

- **Testability** — tests create their own `App` with test doubles
- **Explicit dependencies** — you can see what each component needs
- **No init ordering issues** — construction order is explicit in `New()`
- **Clean shutdown** — the `cleanup` func returned by `New()` handles teardown

---

## Further Reading

- [Go 1.22 Release Notes — Enhanced Routing](https://go.dev/doc/go1.22#enhanced_routing)
- [Effective Go](https://go.dev/doc/effective_go)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [modernc.org/sqlite — Pure-Go SQLite](https://pkg.go.dev/modernc.org/sqlite)
- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
