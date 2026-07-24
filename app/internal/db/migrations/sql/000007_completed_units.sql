-- +goose Up
CREATE TABLE completed_units (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, unit_id)
);

CREATE INDEX completed_units_unit_id_idx ON completed_units (unit_id);

-- +goose Down
DROP TABLE completed_units;
