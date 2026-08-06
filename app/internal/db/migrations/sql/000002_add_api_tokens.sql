-- +goose Up

CREATE TABLE api_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL
        CHECK (name <> '' AND name = btrim(name) AND char_length(name) <= 100),
    token_hash TEXT NOT NULL UNIQUE CHECK (char_length(token_hash) = 64),
    token_prefix TEXT NOT NULL CHECK (token_prefix LIKE 'uc_api_%'),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX api_tokens_user_id_created_at_idx
    ON api_tokens (user_id, created_at DESC);

-- +goose Down
DROP TABLE api_tokens;
