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
        scope IN (
            'login_ip',
            'registration_ip',
            'password_reset_request_ip',
            'password_reset_attempt_ip'
        )
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

CREATE INDEX curriculum_proposals_base_proposal_id_idx
    ON curriculum_proposals (base_proposal_id);
CREATE INDEX curriculum_proposals_status_created_at_idx
    ON curriculum_proposals (status, created_at DESC);

CREATE TABLE curriculum_proposal_authors (
    proposal_id BIGINT NOT NULL REFERENCES curriculum_proposals(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    PRIMARY KEY (proposal_id, user_id)
);

CREATE INDEX curriculum_proposal_authors_user_id_idx
    ON curriculum_proposal_authors (user_id, proposal_id);

CREATE TABLE curriculum_proposal_changes (
    id BIGSERIAL PRIMARY KEY,
    proposal_id BIGINT NOT NULL REFERENCES curriculum_proposals(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'create_unit',
            'rename_unit',
            'update_content',
            'delete_unit',
            'add_dependency',
            'remove_dependency',
            'recognition'
        )
    )
);

CREATE TABLE curriculum_unit_creations (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content))
);

CREATE TABLE curriculum_unit_renames (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name))
);

CREATE TABLE curriculum_unit_content_updates (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    content TEXT NOT NULL CHECK (content <> '' AND content = btrim(content))
);

CREATE TABLE curriculum_unit_deletions (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT
);

CREATE TABLE curriculum_dependency_additions (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    prerequisite_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    CHECK (unit_id <> prerequisite_id)
);

CREATE TABLE curriculum_dependency_removals (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    prerequisite_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    CHECK (unit_id <> prerequisite_id)
);

CREATE TABLE curriculum_recognitions (
    change_id BIGINT PRIMARY KEY REFERENCES curriculum_proposal_changes(id) ON DELETE CASCADE
);

CREATE TABLE curriculum_recognition_sources (
    recognition_change_id BIGINT NOT NULL
        REFERENCES curriculum_recognitions(change_id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    PRIMARY KEY (recognition_change_id, unit_id)
);

CREATE INDEX curriculum_recognition_sources_unit_id_idx
    ON curriculum_recognition_sources (unit_id);

CREATE TABLE curriculum_recognition_targets (
    recognition_change_id BIGINT NOT NULL
        REFERENCES curriculum_recognitions(change_id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    PRIMARY KEY (recognition_change_id, unit_id)
);

CREATE INDEX curriculum_recognition_targets_unit_id_idx
    ON curriculum_recognition_targets (unit_id);

CREATE VIEW curriculum_proposal_change_details AS
SELECT change.id, change.proposal_id, change.kind,
       creation.change_id AS unit_id, creation.name AS unit_name,
       creation.content AS unit_content,
       NULL::BIGINT AS prerequisite_id
FROM curriculum_proposal_changes change
JOIN curriculum_unit_creations creation ON creation.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.kind,
       rename.unit_id, rename.name,
       NULL::TEXT, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_unit_renames rename ON rename.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.kind,
       content_update.unit_id, NULL::TEXT,
       content_update.content, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_unit_content_updates content_update ON content_update.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.kind,
       deletion.unit_id, NULL::TEXT,
       NULL::TEXT, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_unit_deletions deletion ON deletion.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.kind,
       addition.unit_id, NULL::TEXT,
       NULL::TEXT, addition.prerequisite_id
FROM curriculum_proposal_changes change
JOIN curriculum_dependency_additions addition ON addition.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.kind,
       removal.unit_id, NULL::TEXT,
       NULL::TEXT, removal.prerequisite_id
FROM curriculum_proposal_changes change
JOIN curriculum_dependency_removals removal ON removal.change_id = change.id
UNION ALL
SELECT change.id, change.proposal_id, change.kind,
       NULL::BIGINT, NULL::TEXT,
       NULL::TEXT, NULL::BIGINT
FROM curriculum_proposal_changes change
JOIN curriculum_recognitions recognition ON recognition.change_id = change.id;

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
CREATE FUNCTION validate_curriculum_proposal_base() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    base_status TEXT;
BEGIN
    IF NEW.base_proposal_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT status INTO base_status
    FROM curriculum_proposals
    WHERE id = NEW.base_proposal_id;
    IF base_status IS DISTINCT FROM 'accepted' THEN
        RAISE EXCEPTION 'curriculum proposal base % must be accepted', NEW.base_proposal_id;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_proposals_require_accepted_base
BEFORE INSERT OR UPDATE OF base_proposal_id ON curriculum_proposals
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_base();

-- +goose StatementBegin
CREATE FUNCTION validate_curriculum_proposal_acceptance() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    current_proposal_id BIGINT;
BEGIN
    IF NEW.status <> 'accepted' OR OLD.status = 'accepted' THEN
        RETURN NEW;
    END IF;
    SELECT proposal_id INTO current_proposal_id
    FROM curriculum_projection_state
    WHERE singleton = TRUE
    FOR UPDATE;
    IF NEW.base_proposal_id IS DISTINCT FROM current_proposal_id THEN
        RAISE EXCEPTION 'accepted curriculum proposal must extend the current projection';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_proposals_accept_current_base
BEFORE UPDATE OF status ON curriculum_proposals
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_acceptance();

-- +goose StatementBegin
CREATE FUNCTION validate_accepted_curriculum_proposal_is_projected() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status = 'accepted' AND OLD.status <> 'accepted'
       AND NOT EXISTS (
           SELECT 1 FROM curriculum_projection_state
           WHERE singleton = TRUE AND proposal_id = NEW.id
       ) THEN
        RAISE EXCEPTION 'accepted curriculum proposal % must become the current projection', NEW.id;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER curriculum_proposals_require_projection
AFTER UPDATE OF status ON curriculum_proposals
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_accepted_curriculum_proposal_is_projected();

-- +goose StatementBegin
CREATE FUNCTION protect_curriculum_projection_state() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    projected_base_id BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'curriculum projection state cannot be deleted';
    END IF;
    IF NEW.proposal_id IS NULL AND OLD.proposal_id IS NOT NULL THEN
        RAISE EXCEPTION 'curriculum projection cannot discard its accepted history';
    END IF;
    IF NEW.proposal_id IS NOT NULL THEN
        SELECT base_proposal_id INTO projected_base_id
        FROM curriculum_proposals
        WHERE id = NEW.proposal_id AND status = 'accepted';
        IF NOT FOUND THEN
            RAISE EXCEPTION 'curriculum projection must reference an accepted proposal';
        END IF;
        IF projected_base_id IS DISTINCT FROM OLD.proposal_id THEN
            RAISE EXCEPTION 'curriculum projection must advance to a direct successor';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_projection_state_valid
BEFORE UPDATE OR DELETE ON curriculum_projection_state
FOR EACH ROW EXECUTE FUNCTION protect_curriculum_projection_state();

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
CREATE FUNCTION protect_accepted_curriculum_proposal_author() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    parent_status TEXT;
BEGIN
    SELECT status INTO parent_status
    FROM curriculum_proposals
    WHERE id = CASE WHEN TG_OP = 'DELETE' THEN OLD.proposal_id ELSE NEW.proposal_id END;
    IF parent_status = 'accepted' THEN
        RAISE EXCEPTION 'authors of accepted curriculum proposals are immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_proposal_authors_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_proposal_authors
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_author();

-- +goose StatementBegin
CREATE FUNCTION validate_curriculum_proposal_has_author() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    checked_proposal_id BIGINT;
BEGIN
    IF TG_TABLE_NAME = 'curriculum_proposals' THEN
        checked_proposal_id := NEW.id;
    ELSE
        checked_proposal_id := OLD.proposal_id;
    END IF;
    IF EXISTS (SELECT 1 FROM curriculum_proposals WHERE id = checked_proposal_id)
       AND NOT EXISTS (
           SELECT 1 FROM curriculum_proposal_authors
           WHERE proposal_id = checked_proposal_id
       ) THEN
        RAISE EXCEPTION 'curriculum proposal % must have at least one author', checked_proposal_id;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER curriculum_proposals_require_author
AFTER INSERT OR UPDATE ON curriculum_proposals
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_has_author();
CREATE CONSTRAINT TRIGGER curriculum_proposal_authors_preserve_author
AFTER DELETE ON curriculum_proposal_authors
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_has_author();

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
CREATE TRIGGER curriculum_recognitions_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_recognitions
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_proposal_change_detail();

-- +goose StatementBegin
CREATE FUNCTION protect_accepted_curriculum_recognition_member() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    old_parent_status TEXT;
    new_parent_status TEXT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        SELECT proposal.status INTO old_parent_status
        FROM curriculum_recognitions recognition
        JOIN curriculum_proposal_changes change ON change.id = recognition.change_id
        JOIN curriculum_proposals proposal ON proposal.id = change.proposal_id
        WHERE recognition.change_id = OLD.recognition_change_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        SELECT proposal.status INTO new_parent_status
        FROM curriculum_recognitions recognition
        JOIN curriculum_proposal_changes change ON change.id = recognition.change_id
        JOIN curriculum_proposals proposal ON proposal.id = change.proposal_id
        WHERE recognition.change_id = NEW.recognition_change_id;
    END IF;
    IF old_parent_status = 'accepted' OR new_parent_status = 'accepted' THEN
        RAISE EXCEPTION 'members of accepted recognitions are immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER curriculum_recognition_sources_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_recognition_sources
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_recognition_member();
CREATE TRIGGER curriculum_recognition_targets_accepted_immutable
BEFORE INSERT OR UPDATE OR DELETE ON curriculum_recognition_targets
FOR EACH ROW EXECUTE FUNCTION protect_accepted_curriculum_recognition_member();

-- +goose StatementBegin
CREATE FUNCTION validate_curriculum_proposal_change_detail() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    detail_count INTEGER;
    matching_detail_count INTEGER;
BEGIN
    SELECT
        (SELECT count(*) FROM curriculum_unit_creations WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_unit_renames WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_unit_content_updates WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_unit_deletions WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_dependency_additions WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_dependency_removals WHERE change_id = NEW.id)
        + (SELECT count(*) FROM curriculum_recognitions WHERE change_id = NEW.id)
    INTO detail_count;
    SELECT CASE NEW.kind
        WHEN 'create_unit' THEN (SELECT count(*) FROM curriculum_unit_creations WHERE change_id = NEW.id)
        WHEN 'rename_unit' THEN (SELECT count(*) FROM curriculum_unit_renames WHERE change_id = NEW.id)
        WHEN 'update_content' THEN (SELECT count(*) FROM curriculum_unit_content_updates WHERE change_id = NEW.id)
        WHEN 'delete_unit' THEN (SELECT count(*) FROM curriculum_unit_deletions WHERE change_id = NEW.id)
        WHEN 'add_dependency' THEN (SELECT count(*) FROM curriculum_dependency_additions WHERE change_id = NEW.id)
        WHEN 'remove_dependency' THEN (SELECT count(*) FROM curriculum_dependency_removals WHERE change_id = NEW.id)
        WHEN 'recognition' THEN (SELECT count(*) FROM curriculum_recognitions WHERE change_id = NEW.id)
        ELSE 0
    END INTO matching_detail_count;
    IF detail_count <> 1 OR matching_detail_count <> 1 THEN
        RAISE EXCEPTION 'curriculum proposal change % must have exactly one detail row matching kind %', NEW.id, NEW.kind;
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
        + (SELECT count(*) FROM curriculum_recognitions WHERE change_id = OLD.change_id)
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
CREATE CONSTRAINT TRIGGER curriculum_recognitions_preserve_change_detail
AFTER DELETE ON curriculum_recognitions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_proposal_change_after_detail_delete();

-- +goose StatementBegin
CREATE FUNCTION validate_curriculum_recognition() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    recognition_id BIGINT;
BEGIN
    IF TG_TABLE_NAME = 'curriculum_recognitions' THEN
        recognition_id := NEW.change_id;
    ELSE
        recognition_id := OLD.recognition_change_id;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM curriculum_recognitions WHERE change_id = recognition_id
    ) THEN
        RETURN NULL;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM curriculum_recognition_sources
        WHERE recognition_change_id = recognition_id
    ) OR NOT EXISTS (
        SELECT 1 FROM curriculum_recognition_targets
        WHERE recognition_change_id = recognition_id
    ) THEN
        RAISE EXCEPTION 'recognition % must have at least one source and one target', recognition_id;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER curriculum_recognitions_require_members
AFTER INSERT OR UPDATE ON curriculum_recognitions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_recognition();
CREATE CONSTRAINT TRIGGER curriculum_recognition_sources_preserve_members
AFTER UPDATE OR DELETE ON curriculum_recognition_sources
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_recognition();
CREATE CONSTRAINT TRIGGER curriculum_recognition_targets_preserve_members
AFTER UPDATE OR DELETE ON curriculum_recognition_targets
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_curriculum_recognition();

CREATE TABLE learning_paths (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name <> '' AND name = btrim(name)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX learning_paths_user_id_idx
    ON learning_paths (user_id, updated_at DESC);

CREATE TABLE learning_path_units (
    path_id BIGINT NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    PRIMARY KEY (path_id, unit_id)
);

CREATE TABLE unit_completion_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES curriculum_unit_creations(change_id) ON DELETE RESTRICT,
    curriculum_proposal_id BIGINT NOT NULL REFERENCES curriculum_proposals(id) ON DELETE RESTRICT,
    is_completed BOOLEAN NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX unit_completion_events_current_idx
    ON unit_completion_events (user_id, unit_id, id DESC);
CREATE INDEX unit_completion_events_unit_id_idx
    ON unit_completion_events (unit_id);
CREATE INDEX unit_completion_events_proposal_id_idx
    ON unit_completion_events (curriculum_proposal_id);

-- +goose StatementBegin
CREATE FUNCTION validate_unit_completion_event() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        RAISE EXCEPTION 'unit completion events are immutable';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM curriculum_proposals
        WHERE id = NEW.curriculum_proposal_id AND status = 'accepted'
    ) THEN
        RAISE EXCEPTION 'unit completion event must reference an accepted curriculum state';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER unit_completion_events_valid
BEFORE INSERT OR UPDATE ON unit_completion_events
FOR EACH ROW EXECUTE FUNCTION validate_unit_completion_event();

-- +goose Down
DROP TABLE unit_completion_events;
DROP FUNCTION validate_unit_completion_event();
DROP TABLE learning_path_units;
DROP TABLE learning_paths;
DROP TABLE curriculum_projection_state;
DROP FUNCTION protect_curriculum_projection_state();
DROP TABLE unit_dependencies;
DROP TABLE units;
DROP VIEW curriculum_proposal_change_details;
DROP TABLE curriculum_recognition_targets;
DROP TABLE curriculum_recognition_sources;
DROP TABLE curriculum_recognitions;
DROP TABLE curriculum_dependency_removals;
DROP TABLE curriculum_dependency_additions;
DROP TABLE curriculum_unit_deletions;
DROP TABLE curriculum_unit_content_updates;
DROP TABLE curriculum_unit_renames;
DROP TABLE curriculum_unit_creations;
DROP TRIGGER curriculum_proposal_changes_require_detail ON curriculum_proposal_changes;
DROP FUNCTION validate_curriculum_recognition();
DROP FUNCTION validate_curriculum_proposal_change_after_detail_delete();
DROP FUNCTION validate_curriculum_proposal_change_detail();
DROP FUNCTION protect_accepted_curriculum_recognition_member();
DROP FUNCTION protect_accepted_curriculum_proposal_change_detail();
DROP TABLE curriculum_proposal_changes;
DROP TRIGGER curriculum_proposals_require_projection ON curriculum_proposals;
DROP TRIGGER curriculum_proposals_accept_current_base ON curriculum_proposals;
DROP TRIGGER curriculum_proposals_require_accepted_base ON curriculum_proposals;
DROP TRIGGER curriculum_proposals_accepted_immutable ON curriculum_proposals;
DROP TRIGGER curriculum_proposals_require_author ON curriculum_proposals;
DROP TABLE curriculum_proposal_authors;
DROP FUNCTION validate_curriculum_proposal_has_author();
DROP FUNCTION protect_accepted_curriculum_proposal_author();
DROP FUNCTION validate_accepted_curriculum_proposal_is_projected();
DROP FUNCTION validate_curriculum_proposal_acceptance();
DROP FUNCTION validate_curriculum_proposal_base();
DROP FUNCTION protect_accepted_curriculum_proposal_change();
DROP FUNCTION protect_accepted_curriculum_proposal();
DROP TABLE curriculum_proposals;
DROP TABLE authentication_rate_limits;
DROP TABLE password_reset_tokens;
DROP TABLE sessions;
DROP TABLE local_authentications;
DROP TABLE users;
