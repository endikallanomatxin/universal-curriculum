-- +goose Up

CREATE TABLE api_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL
        CHECK (name <> '' AND name = btrim(name) AND char_length(name) <= 100),
    token_hash TEXT NOT NULL UNIQUE CHECK (char_length(token_hash) = 64),
    token_prefix TEXT NOT NULL CHECK (token_prefix LIKE 'uc_api_%'),
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX api_tokens_user_id_created_at_idx
    ON api_tokens (user_id, created_at DESC);

CREATE TABLE oauth_authorization_codes (
    code_hash TEXT PRIMARY KEY CHECK (char_length(code_hash) = 64),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL CHECK (client_id <> ''),
    client_name TEXT NOT NULL CHECK (client_name <> ''),
    redirect_uri TEXT NOT NULL CHECK (redirect_uri <> ''),
    resource TEXT NOT NULL CHECK (resource <> ''),
    scope TEXT NOT NULL CHECK (scope = 'mcp'),
    code_challenge TEXT NOT NULL CHECK (char_length(code_challenge) = 43),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX oauth_authorization_codes_expires_at_idx
    ON oauth_authorization_codes (expires_at);

CREATE TABLE oauth_connections (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL CHECK (client_id <> ''),
    client_name TEXT NOT NULL CHECK (client_name <> ''),
    resource TEXT NOT NULL CHECK (resource <> ''),
    authorized_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    UNIQUE (user_id, client_id, resource)
);

CREATE TABLE oauth_access_tokens (
    token_hash TEXT PRIMARY KEY CHECK (char_length(token_hash) = 64),
    connection_id BIGINT NOT NULL REFERENCES oauth_connections(id) ON DELETE CASCADE,
    scope TEXT NOT NULL CHECK (scope = 'mcp'),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX oauth_access_tokens_expires_at_idx ON oauth_access_tokens (expires_at);

CREATE TABLE oauth_refresh_tokens (
    token_hash TEXT PRIMARY KEY CHECK (char_length(token_hash) = 64),
    connection_id BIGINT NOT NULL REFERENCES oauth_connections(id) ON DELETE CASCADE,
    scope TEXT NOT NULL CHECK (scope = 'mcp'),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX oauth_refresh_tokens_expires_at_idx ON oauth_refresh_tokens (expires_at);

-- +goose Down
DROP TABLE oauth_refresh_tokens;
DROP TABLE oauth_access_tokens;
DROP TABLE oauth_connections;
DROP TABLE oauth_authorization_codes;
DROP TABLE api_tokens;
