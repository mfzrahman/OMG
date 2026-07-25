CREATE TABLE IF NOT EXISTS routes (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    path            TEXT NOT NULL,
    methods         TEXT NOT NULL DEFAULT '["GET"]',
    rewrite_pattern TEXT NOT NULL DEFAULT '',
    headers         TEXT NOT NULL DEFAULT '{}',
    timeout_ns      INTEGER NOT NULL DEFAULT 30000000000,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
    updated_at      INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
);

CREATE TABLE IF NOT EXISTS backends (
    id          TEXT PRIMARY KEY,
    route_id    TEXT NOT NULL,
    url         TEXT NOT NULL,
    weight      INTEGER NOT NULL DEFAULT 1,
    health_path TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS auth_configs (
    id       TEXT PRIMARY KEY,
    route_id TEXT NOT NULL UNIQUE,
    type     TEXT NOT NULL DEFAULT 'api_key',
    issuer   TEXT NOT NULL DEFAULT '',
    secret   TEXT NOT NULL DEFAULT '',
    api_key  TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rate_limits (
    id         TEXT PRIMARY KEY,
    route_id   TEXT NOT NULL UNIQUE,
    requests   INTEGER NOT NULL DEFAULT 100,
    window_ns  INTEGER NOT NULL DEFAULT 60000000000,
    per_client INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_backends_route ON backends (route_id);
CREATE INDEX IF NOT EXISTS idx_auth_configs_route ON auth_configs (route_id);
CREATE INDEX IF NOT EXISTS idx_rate_limits_route ON rate_limits (route_id);
