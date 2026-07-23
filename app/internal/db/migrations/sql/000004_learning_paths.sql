-- +goose Up
CREATE TABLE learning_paths (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    description TEXT NOT NULL DEFAULT '' CHECK (description = btrim(description)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX learning_paths_user_id_idx ON learning_paths (user_id, updated_at DESC);

CREATE TABLE learning_path_units (
    path_id BIGINT NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    position INTEGER NOT NULL CHECK (position > 0),
    PRIMARY KEY (path_id, unit_id),
    UNIQUE (path_id, position)
);

-- +goose Down
DROP TABLE learning_path_units;
DROP TABLE learning_paths;
