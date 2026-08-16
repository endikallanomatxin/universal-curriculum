-- +goose Up

SELECT pg_advisory_xact_lock(843015922);
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

CREATE INDEX units_name_trgm_idx
    ON units USING GIN (lower(name) public.gin_trgm_ops);
CREATE INDEX units_content_trgm_idx
    ON units USING GIN (lower(content) public.gin_trgm_ops);

-- +goose Down

DROP INDEX units_content_trgm_idx;
DROP INDEX units_name_trgm_idx;
