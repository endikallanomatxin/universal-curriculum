-- +goose Up

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    full_name TEXT NOT NULL
        CHECK (full_name <> '' AND full_name = btrim(full_name) AND char_length(full_name) <= 200),
    alias TEXT CHECK (alias IS NULL OR alias <> ''),
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX users_alias_unique
    ON users (lower(alias))
    WHERE alias IS NOT NULL;

CREATE TABLE local_authentications (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL
        CHECK (
            email <> ''
            AND email = lower(email)
            AND email = btrim(email)
            AND char_length(email) <= 320
        ),
    password_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX local_authentications_email_unique
    ON local_authentications (lower(email));

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    last_activity_at TIMESTAMPTZ NOT NULL,
    rotated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE password_reset_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES local_authentications(user_id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX password_reset_tokens_expires_at_idx
    ON password_reset_tokens (expires_at);

CREATE TABLE authentication_rate_limits (
    scope TEXT NOT NULL CHECK (
        scope IN ('password_reset_request_ip', 'password_reset_attempt_ip')
    ),
    key TEXT NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL,
    event_count INTEGER NOT NULL CHECK (event_count >= 0),
    blocked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope, key)
);

CREATE INDEX authentication_rate_limits_cleanup_idx
    ON authentication_rate_limits (updated_at, blocked_until);

CREATE SEQUENCE curriculum_unit_ids;

CREATE TABLE units (
    id BIGINT PRIMARY KEY DEFAULT nextval('curriculum_unit_ids'),
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER SEQUENCE curriculum_unit_ids OWNED BY units.id;

CREATE TABLE unit_dependencies (
    unit_id BIGINT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    prerequisite_id BIGINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (unit_id, prerequisite_id),
    CHECK (unit_id <> prerequisite_id)
);

CREATE INDEX unit_dependencies_prerequisite_id_idx
    ON unit_dependencies (prerequisite_id);

CREATE TABLE curriculum_proposals (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (title <> '' AND title = btrim(title)),
    rationale TEXT NOT NULL CHECK (rationale <> '' AND rationale = btrim(rationale)),
    status TEXT NOT NULL CHECK (status IN ('draft', 'accepted', 'rejected')),
    base_proposal_id BIGINT REFERENCES curriculum_proposals(id) ON DELETE RESTRICT,
    reverts_proposal_id BIGINT UNIQUE REFERENCES curriculum_proposals(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    CHECK (base_proposal_id IS NULL OR base_proposal_id <> id),
    CHECK (
        (status = 'accepted' AND accepted_at IS NOT NULL)
        OR
        (status <> 'accepted' AND accepted_at IS NULL)
    )
);

CREATE TABLE curriculum_proposal_changes (
    id BIGSERIAL PRIMARY KEY,
    proposal_id BIGINT NOT NULL REFERENCES curriculum_proposals(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position > 0),
    kind TEXT NOT NULL CHECK (
        kind IN (
            'create_unit',
            'update_unit',
            'update_content',
            'delete_unit',
            'add_dependency',
            'remove_dependency'
        )
    ),
    unit_id BIGINT NOT NULL CHECK (unit_id > 0),
    unit_name TEXT,
    previous_unit_name TEXT,
    unit_content TEXT,
    previous_unit_content TEXT,
    prerequisite_id BIGINT CHECK (prerequisite_id > 0),
    UNIQUE (proposal_id, position),
    CHECK (
        (
            kind = 'create_unit'
            AND unit_name IS NOT NULL AND unit_name <> ''
            AND unit_content IS NOT NULL AND unit_content <> ''
            AND previous_unit_name IS NULL
            AND previous_unit_content IS NULL
            AND prerequisite_id IS NULL
        )
        OR
        (
            kind = 'update_unit'
            AND unit_name IS NOT NULL AND unit_name <> ''
            AND previous_unit_name IS NOT NULL AND previous_unit_name <> ''
            AND unit_content IS NULL
            AND previous_unit_content IS NULL
            AND prerequisite_id IS NULL
        )
        OR
        (
            kind = 'update_content'
            AND unit_name IS NULL
            AND previous_unit_name IS NULL
            AND unit_content IS NOT NULL AND unit_content <> ''
            AND previous_unit_content IS NOT NULL AND previous_unit_content <> ''
            AND prerequisite_id IS NULL
        )
        OR
        (
            kind = 'delete_unit'
            AND unit_name IS NOT NULL AND unit_name <> ''
            AND unit_content IS NOT NULL AND unit_content <> ''
            AND previous_unit_name IS NULL
            AND previous_unit_content IS NULL
            AND prerequisite_id IS NULL
        )
        OR
        (
            kind IN ('add_dependency', 'remove_dependency')
            AND unit_name IS NULL
            AND previous_unit_name IS NULL
            AND unit_content IS NULL
            AND previous_unit_content IS NULL
            AND prerequisite_id IS NOT NULL
            AND prerequisite_id <> unit_id
        )
    )
);

CREATE TABLE curriculum_projection_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    proposal_id BIGINT REFERENCES curriculum_proposals(id) ON DELETE RESTRICT
);

INSERT INTO curriculum_projection_state (singleton, proposal_id)
VALUES (TRUE, NULL);

-- +goose StatementBegin
CREATE FUNCTION protect_accepted_curriculum_proposal() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'accepted' THEN
        RAISE EXCEPTION 'accepted curriculum proposals are immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_proposals_accepted_immutable
BEFORE UPDATE OR DELETE ON curriculum_proposals
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal();

-- +goose StatementBegin
CREATE FUNCTION protect_accepted_curriculum_proposal_change() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    parent_status TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        SELECT status INTO parent_status FROM curriculum_proposals WHERE id = OLD.proposal_id;
    ELSE
        SELECT status INTO parent_status FROM curriculum_proposals WHERE id = NEW.proposal_id;
    END IF;
    IF parent_status = 'accepted' THEN
        RAISE EXCEPTION 'changes of accepted curriculum proposals are immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_proposal_changes_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_proposal_changes
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change();

CREATE TABLE learning_paths (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    description TEXT NOT NULL DEFAULT '' CHECK (description = btrim(description)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX learning_paths_user_id_idx
    ON learning_paths (user_id, updated_at DESC);

CREATE TABLE learning_path_units (
    path_id BIGINT NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    position INTEGER NOT NULL CHECK (position > 0),
    PRIMARY KEY (path_id, unit_id),
    UNIQUE (path_id, position)
);

CREATE TABLE completed_units (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, unit_id)
);

CREATE INDEX completed_units_unit_id_idx ON completed_units (unit_id);

-- +goose Down
DROP TABLE completed_units;
DROP TABLE learning_path_units;
DROP TABLE learning_paths;
DROP TABLE curriculum_projection_state;
DROP TABLE curriculum_proposal_changes;
DROP TRIGGER curriculum_proposals_accepted_immutable ON curriculum_proposals;
DROP FUNCTION protect_accepted_curriculum_proposal_change();
DROP FUNCTION protect_accepted_curriculum_proposal();
DROP TABLE curriculum_proposals;
DROP TABLE unit_dependencies;
DROP TABLE units;
DROP TABLE authentication_rate_limits;
DROP TABLE password_reset_tokens;
DROP TABLE sessions;
DROP TABLE local_authentications;
DROP TABLE users;
