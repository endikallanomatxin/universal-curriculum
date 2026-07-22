-- +goose Up
CREATE TABLE units (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    description TEXT NOT NULL CHECK (description <> '' AND description = btrim(description)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE unit_dependencies (
    unit_id BIGINT NOT NULL,
    prerequisite_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unit_dependencies_primary_key PRIMARY KEY (unit_id, prerequisite_id),
    CONSTRAINT unit_dependencies_not_self CHECK (unit_id <> prerequisite_id),
    CONSTRAINT unit_dependencies_unit_foreign_key
        FOREIGN KEY (unit_id) REFERENCES units(id) ON DELETE CASCADE,
    CONSTRAINT unit_dependencies_prerequisite_foreign_key
        FOREIGN KEY (prerequisite_id) REFERENCES units(id) ON DELETE RESTRICT
);

CREATE INDEX unit_dependencies_prerequisite_id_idx
    ON unit_dependencies (prerequisite_id);

-- +goose Down
DROP TABLE unit_dependencies;
DROP TABLE units;
