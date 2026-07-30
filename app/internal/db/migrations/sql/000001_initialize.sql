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

CREATE TABLE curriculum_proposals (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (title <> '' AND title = btrim(title)),
    rationale TEXT NOT NULL CHECK (rationale <> '' AND rationale = btrim(rationale)),
    status TEXT NOT NULL CHECK (status IN ('draft', 'accepted', 'rejected')),
    base_proposal_id BIGINT REFERENCES curriculum_proposals(id) ON DELETE RESTRICT,
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
            'rename_unit',
            'update_content',
            'delete_unit',
            'add_dependency',
            'remove_dependency'
        )
    ),
    UNIQUE (proposal_id, position),
    UNIQUE (id, kind)
);

CREATE TABLE curriculum_unit_creations (
    change_id BIGINT PRIMARY KEY,
    change_kind TEXT NOT NULL DEFAULT 'create_unit' CHECK (change_kind = 'create_unit'),
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content)),
    FOREIGN KEY (change_id, change_kind)
        REFERENCES curriculum_proposal_changes(id, kind) ON DELETE CASCADE
);

CREATE TABLE curriculum_unit_renames (
    change_id BIGINT PRIMARY KEY,
    change_kind TEXT NOT NULL DEFAULT 'rename_unit' CHECK (change_kind = 'rename_unit'),
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    previous_name TEXT NOT NULL CHECK (previous_name <> '' AND previous_name = btrim(previous_name)),
    FOREIGN KEY (change_id, change_kind)
        REFERENCES curriculum_proposal_changes(id, kind) ON DELETE CASCADE
);

CREATE TABLE curriculum_unit_content_updates (
    change_id BIGINT PRIMARY KEY,
    change_kind TEXT NOT NULL DEFAULT 'update_content' CHECK (change_kind = 'update_content'),
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content)),
    previous_content TEXT NOT NULL
        CHECK (previous_content <> '' AND previous_content = btrim(previous_content)),
    FOREIGN KEY (change_id, change_kind)
        REFERENCES curriculum_proposal_changes(id, kind) ON DELETE CASCADE
);

CREATE TABLE curriculum_unit_deletions (
    change_id BIGINT PRIMARY KEY,
    change_kind TEXT NOT NULL DEFAULT 'delete_unit' CHECK (change_kind = 'delete_unit'),
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content)),
    FOREIGN KEY (change_id, change_kind)
        REFERENCES curriculum_proposal_changes(id, kind) ON DELETE CASCADE
);

CREATE TABLE curriculum_dependency_additions (
    change_id BIGINT PRIMARY KEY,
    change_kind TEXT NOT NULL DEFAULT 'add_dependency' CHECK (change_kind = 'add_dependency'),
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    prerequisite_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    CHECK (unit_id <> prerequisite_id),
    FOREIGN KEY (change_id, change_kind)
        REFERENCES curriculum_proposal_changes(id, kind) ON DELETE CASCADE
);

CREATE TABLE curriculum_dependency_removals (
    change_id BIGINT PRIMARY KEY,
    change_kind TEXT NOT NULL DEFAULT 'remove_dependency' CHECK (change_kind = 'remove_dependency'),
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    prerequisite_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    CHECK (unit_id <> prerequisite_id),
    FOREIGN KEY (change_id, change_kind)
        REFERENCES curriculum_proposal_changes(id, kind) ON DELETE CASCADE
);

CREATE VIEW curriculum_proposal_change_details AS
SELECT change.id, change.proposal_id, change.position, change.kind,
       creation.change_id AS unit_id, creation.name AS unit_name, NULL::TEXT AS previous_unit_name,
       creation.content AS unit_content, NULL::TEXT AS previous_unit_content,
       NULL::BIGINT AS prerequisite_id
FROM curriculum_proposal_changes change
JOIN curriculum_unit_creations creation ON creation.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.position, change.kind,
       rename.unit_id, rename.name, rename.previous_name,
       NULL::TEXT, NULL::TEXT, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_unit_renames rename ON rename.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.position, change.kind,
       content_update.unit_id, NULL::TEXT, NULL::TEXT,
       content_update.content, content_update.previous_content, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_unit_content_updates content_update ON content_update.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.position, change.kind,
       deletion.unit_id, deletion.name, NULL::TEXT,
       deletion.content, NULL::TEXT, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_unit_deletions deletion ON deletion.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.position, change.kind,
       addition.unit_id, NULL::TEXT, NULL::TEXT,
       NULL::TEXT, NULL::TEXT, addition.prerequisite_id
FROM curriculum_proposal_changes change
JOIN curriculum_dependency_additions addition ON addition.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.position, change.kind,
       removal.unit_id, NULL::TEXT, NULL::TEXT,
       NULL::TEXT, NULL::TEXT, removal.prerequisite_id
FROM curriculum_proposal_changes change
JOIN curriculum_dependency_removals removal ON removal.change_id = change.id;

CREATE TABLE units (
    id BIGINT PRIMARY KEY REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE unit_dependencies (
    unit_id BIGINT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    prerequisite_id BIGINT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (unit_id, prerequisite_id),
    CHECK (unit_id <> prerequisite_id)
);

CREATE INDEX unit_dependencies_prerequisite_id_idx
    ON unit_dependencies (prerequisite_id);

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
    old_parent_status TEXT;
    new_parent_status TEXT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        SELECT status INTO old_parent_status FROM curriculum_proposals WHERE id = OLD.proposal_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        SELECT status INTO new_parent_status FROM curriculum_proposals WHERE id = NEW.proposal_id;
    END IF;
    IF old_parent_status = 'accepted' OR new_parent_status = 'accepted' THEN
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

-- +goose StatementBegin
CREATE FUNCTION protect_accepted_curriculum_proposal_change_detail() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    old_parent_status TEXT;
    new_parent_status TEXT;
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.change_id <> NEW.change_id THEN
        RAISE EXCEPTION 'curriculum proposal change details cannot move between changes';
    END IF;
    IF TG_OP <> 'INSERT' THEN
        SELECT proposal.status INTO old_parent_status
        FROM curriculum_proposal_changes change
        JOIN curriculum_proposals proposal ON proposal.id = change.proposal_id
        WHERE change.id = OLD.change_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        SELECT proposal.status INTO new_parent_status
        FROM curriculum_proposal_changes change
        JOIN curriculum_proposals proposal ON proposal.id = change.proposal_id
        WHERE change.id = NEW.change_id;
    END IF;
    IF old_parent_status = 'accepted' OR new_parent_status = 'accepted' THEN
        RAISE EXCEPTION 'details of accepted curriculum proposals are immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_unit_creations_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_unit_creations
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();
CREATE TRIGGER curriculum_unit_renames_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_unit_renames
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();
CREATE TRIGGER curriculum_unit_content_updates_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_unit_content_updates
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();
CREATE TRIGGER curriculum_unit_deletions_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_unit_deletions
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();
CREATE TRIGGER curriculum_dependency_additions_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_dependency_additions
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();
CREATE TRIGGER curriculum_dependency_removals_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_dependency_removals
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();

-- +goose StatementBegin
CREATE FUNCTION validate_curriculum_proposal_change_detail() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    detail_count INTEGER;
BEGIN
    SELECT
        (SELECT count(*) FROM curriculum_unit_creations WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_unit_renames WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_unit_content_updates WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_unit_deletions WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_dependency_additions WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_dependency_removals WHERE change_id = NEW.id)
    INTO detail_count;
    IF detail_count <> 1 THEN
        RAISE EXCEPTION 'curriculum proposal change % must have exactly one detail row', NEW.id;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER curriculum_proposal_changes_require_detail
AFTER INSERT OR UPDATE ON curriculum_proposal_changes
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_detail();

-- +goose StatementBegin
CREATE FUNCTION validate_curriculum_proposal_change_after_detail_delete() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    detail_count INTEGER;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM curriculum_proposal_changes WHERE id = OLD.change_id
    ) THEN
        RETURN NULL;
    END IF;
    SELECT
        (SELECT count(*) FROM curriculum_unit_creations WHERE change_id = OLD.change_id)
        + (SELECT count(*) FROM curriculum_unit_renames WHERE change_id = OLD.change_id)
        + (SELECT count(*) FROM curriculum_unit_content_updates WHERE change_id = OLD.change_id)
        + (SELECT count(*) FROM curriculum_unit_deletions WHERE change_id = OLD.change_id)
        + (SELECT count(*) FROM curriculum_dependency_additions WHERE change_id = OLD.change_id)
        + (SELECT count(*) FROM curriculum_dependency_removals WHERE change_id = OLD.change_id)
    INTO detail_count;
    IF detail_count <> 1 THEN
        RAISE EXCEPTION 'curriculum proposal change % must have exactly one detail row', OLD.change_id;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER curriculum_unit_creations_preserve_change_detail
AFTER DELETE ON curriculum_unit_creations
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();
CREATE CONSTRAINT TRIGGER curriculum_unit_renames_preserve_change_detail
AFTER DELETE ON curriculum_unit_renames
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();
CREATE CONSTRAINT TRIGGER curriculum_unit_content_updates_preserve_change_detail
AFTER DELETE ON curriculum_unit_content_updates
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();
CREATE CONSTRAINT TRIGGER curriculum_unit_deletions_preserve_change_detail
AFTER DELETE ON curriculum_unit_deletions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();
CREATE CONSTRAINT TRIGGER curriculum_dependency_additions_preserve_change_detail
AFTER DELETE ON curriculum_dependency_additions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();
CREATE CONSTRAINT TRIGGER curriculum_dependency_removals_preserve_change_detail
AFTER DELETE ON curriculum_dependency_removals
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();

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
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    position INTEGER NOT NULL CHECK (position > 0),
    PRIMARY KEY (path_id, unit_id),
    UNIQUE (path_id, position)
);

CREATE TABLE completed_units (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    curriculum_proposal_id BIGINT NOT NULL REFERENCES curriculum_proposals(id) ON DELETE RESTRICT,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, unit_id)
);

CREATE INDEX completed_units_unit_id_idx ON completed_units (unit_id);

-- +goose Down
DROP TABLE completed_units;
DROP TABLE learning_path_units;
DROP TABLE learning_paths;
DROP TABLE curriculum_projection_state;
DROP TABLE unit_dependencies;
DROP TABLE units;
DROP VIEW curriculum_proposal_change_details;
DROP TABLE curriculum_dependency_removals;
DROP TABLE curriculum_dependency_additions;
DROP TABLE curriculum_unit_deletions;
DROP TABLE curriculum_unit_content_updates;
DROP TABLE curriculum_unit_renames;
DROP TABLE curriculum_unit_creations;
DROP TRIGGER curriculum_proposal_changes_require_detail ON curriculum_proposal_changes;
DROP FUNCTION validate_curriculum_proposal_change_after_detail_delete();
DROP FUNCTION validate_curriculum_proposal_change_detail();
DROP FUNCTION protect_accepted_curriculum_proposal_change_detail();
DROP TABLE curriculum_proposal_changes;
DROP TRIGGER curriculum_proposals_accepted_immutable ON curriculum_proposals;
DROP FUNCTION protect_accepted_curriculum_proposal_change();
DROP FUNCTION protect_accepted_curriculum_proposal();
DROP TABLE curriculum_proposals;
DROP TABLE authentication_rate_limits;
DROP TABLE password_reset_tokens;
DROP TABLE sessions;
DROP TABLE local_authentications;
DROP TABLE users;
