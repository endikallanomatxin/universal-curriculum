-- +goose Up

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

ALTER TABLE oauth_authorization_codes ADD COLUMN client_name TEXT;
UPDATE oauth_authorization_codes SET client_name = client_id;
ALTER TABLE oauth_authorization_codes
    ALTER COLUMN client_name SET NOT NULL,
    ADD CONSTRAINT oauth_authorization_codes_client_name_check CHECK (client_name <> '');

INSERT INTO oauth_connections (
    user_id, client_id, client_name, resource, authorized_at, last_used_at
)
SELECT user_id, client_id, client_id, resource, MIN(created_at), MAX(last_used_at)
FROM (
    SELECT user_id, client_id, resource, created_at, last_used_at
    FROM oauth_access_tokens
    UNION ALL
    SELECT user_id, client_id, resource, created_at, NULL::TIMESTAMPTZ
    FROM oauth_refresh_tokens
) tokens
GROUP BY user_id, client_id, resource;

ALTER TABLE oauth_access_tokens
    ADD COLUMN connection_id BIGINT REFERENCES oauth_connections(id) ON DELETE CASCADE;
ALTER TABLE oauth_refresh_tokens
    ADD COLUMN connection_id BIGINT REFERENCES oauth_connections(id) ON DELETE CASCADE;

UPDATE oauth_access_tokens token
SET connection_id = connection.id
FROM oauth_connections connection
WHERE connection.user_id = token.user_id
  AND connection.client_id = token.client_id
  AND connection.resource = token.resource;

UPDATE oauth_refresh_tokens token
SET connection_id = connection.id
FROM oauth_connections connection
WHERE connection.user_id = token.user_id
  AND connection.client_id = token.client_id
  AND connection.resource = token.resource;

ALTER TABLE oauth_access_tokens ALTER COLUMN connection_id SET NOT NULL;
ALTER TABLE oauth_refresh_tokens ALTER COLUMN connection_id SET NOT NULL;

DROP INDEX oauth_access_tokens_user_id_idx;
DROP INDEX oauth_refresh_tokens_user_id_idx;
ALTER TABLE oauth_access_tokens DROP COLUMN user_id, DROP COLUMN client_id, DROP COLUMN resource;
ALTER TABLE oauth_refresh_tokens DROP COLUMN user_id, DROP COLUMN client_id, DROP COLUMN resource;

-- +goose Down

ALTER TABLE oauth_access_tokens ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE oauth_access_tokens ADD COLUMN client_id TEXT;
ALTER TABLE oauth_access_tokens ADD COLUMN resource TEXT;
ALTER TABLE oauth_refresh_tokens ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE oauth_refresh_tokens ADD COLUMN client_id TEXT;
ALTER TABLE oauth_refresh_tokens ADD COLUMN resource TEXT;

UPDATE oauth_access_tokens token
SET user_id = connection.user_id,
    client_id = connection.client_id,
    resource = connection.resource
FROM oauth_connections connection
WHERE connection.id = token.connection_id;

UPDATE oauth_refresh_tokens token
SET user_id = connection.user_id,
    client_id = connection.client_id,
    resource = connection.resource
FROM oauth_connections connection
WHERE connection.id = token.connection_id;

ALTER TABLE oauth_access_tokens
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN client_id SET NOT NULL,
    ALTER COLUMN resource SET NOT NULL,
    ADD CONSTRAINT oauth_access_tokens_client_id_check CHECK (client_id <> ''),
    ADD CONSTRAINT oauth_access_tokens_resource_check CHECK (resource <> '');
ALTER TABLE oauth_refresh_tokens
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN client_id SET NOT NULL,
    ALTER COLUMN resource SET NOT NULL,
    ADD CONSTRAINT oauth_refresh_tokens_client_id_check CHECK (client_id <> ''),
    ADD CONSTRAINT oauth_refresh_tokens_resource_check CHECK (resource <> '');

CREATE INDEX oauth_access_tokens_user_id_idx ON oauth_access_tokens (user_id);
CREATE INDEX oauth_refresh_tokens_user_id_idx ON oauth_refresh_tokens (user_id);
ALTER TABLE oauth_access_tokens DROP COLUMN connection_id;
ALTER TABLE oauth_refresh_tokens DROP COLUMN connection_id;

ALTER TABLE oauth_authorization_codes DROP COLUMN client_name;
DROP TABLE oauth_connections;
