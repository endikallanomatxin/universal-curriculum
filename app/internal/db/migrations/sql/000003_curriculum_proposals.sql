-- +goose Up
CREATE SEQUENCE curriculum_unit_ids;
SELECT setval(
    'curriculum_unit_ids',
    COALESCE((SELECT MAX(id) FROM units), 0) + 1,
    FALSE
);

CREATE TABLE curriculum_proposals (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (title <> '' AND title = btrim(title)),
    rationale TEXT NOT NULL CHECK (rationale <> '' AND rationale = btrim(rationale)),
    status TEXT NOT NULL CHECK (status IN ('draft', 'accepted', 'rejected')),
    base_version BIGINT NOT NULL CHECK (base_version >= 0),
    published_version BIGINT UNIQUE,
    reverts_proposal_id BIGINT UNIQUE REFERENCES curriculum_proposals(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    CHECK (
        (status = 'accepted' AND accepted_at IS NOT NULL AND published_version IS NOT NULL AND published_version > base_version)
        OR
        (status <> 'accepted' AND accepted_at IS NULL AND published_version IS NULL)
    )
);

CREATE TABLE curriculum_proposal_changes (
    id BIGSERIAL PRIMARY KEY,
    proposal_id BIGINT NOT NULL REFERENCES curriculum_proposals(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position > 0),
    kind TEXT NOT NULL CHECK (kind IN ('create_unit', 'update_unit', 'delete_unit', 'add_dependency', 'remove_dependency')),
    unit_id BIGINT NOT NULL CHECK (unit_id > 0),
    unit_name TEXT,
    unit_description TEXT,
    previous_unit_name TEXT,
    previous_unit_description TEXT,
    prerequisite_id BIGINT CHECK (prerequisite_id > 0),
    UNIQUE (proposal_id, position),
    CHECK (
        (kind = 'create_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND prerequisite_id IS NULL)
        OR
        (kind = 'update_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND previous_unit_name IS NOT NULL AND previous_unit_name <> '' AND previous_unit_description IS NOT NULL AND previous_unit_description <> '' AND prerequisite_id IS NULL)
        OR
        (kind = 'delete_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND prerequisite_id IS NULL)
        OR
        (kind IN ('add_dependency', 'remove_dependency') AND unit_name IS NULL AND unit_description IS NULL AND previous_unit_name IS NULL AND previous_unit_description IS NULL AND prerequisite_id IS NOT NULL AND prerequisite_id <> unit_id)
    )
);

CREATE TABLE curriculum_projection_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    version BIGINT NOT NULL CHECK (version >= 0),
    proposal_id BIGINT REFERENCES curriculum_proposals(id) ON DELETE RESTRICT
);

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

INSERT INTO curriculum_proposals (
    title, rationale, status, base_version
)
VALUES (
    'Initial curriculum snapshot',
    'Imported the curriculum that existed before proposal-backed publishing.',
    'draft',
    0
);

INSERT INTO curriculum_proposal_changes (
    proposal_id, position, kind, unit_id, unit_name, unit_description
)
SELECT
    currval('curriculum_proposals_id_seq'),
    ROW_NUMBER() OVER (ORDER BY unit.id),
    'create_unit',
    unit.id,
    unit.name,
    unit.description
FROM units unit;

INSERT INTO curriculum_proposal_changes (
    proposal_id, position, kind, unit_id, prerequisite_id
)
SELECT
    currval('curriculum_proposals_id_seq'),
    (SELECT COUNT(*) FROM units) + ROW_NUMBER() OVER (ORDER BY dependency.unit_id, dependency.prerequisite_id),
    'add_dependency',
    dependency.unit_id,
    dependency.prerequisite_id
FROM unit_dependencies dependency;

UPDATE curriculum_proposals
SET status = 'accepted', published_version = 1, accepted_at = NOW()
WHERE id = currval('curriculum_proposals_id_seq');

INSERT INTO curriculum_projection_state (version, proposal_id)
VALUES (1, currval('curriculum_proposals_id_seq'));

-- +goose Down
DROP TABLE curriculum_projection_state;
DROP TABLE curriculum_proposal_changes;
DROP TABLE curriculum_proposals;
DROP FUNCTION protect_accepted_curriculum_proposal_change();
DROP FUNCTION protect_accepted_curriculum_proposal();
DROP SEQUENCE curriculum_unit_ids;
