-- +goose Up
ALTER TABLE units
ADD COLUMN content TEXT NOT NULL DEFAULT '';

UPDATE units
SET content = description
WHERE content = '';

ALTER TABLE units
ADD CONSTRAINT units_content_not_empty
CHECK (content <> '' AND content = btrim(content));

ALTER TABLE curriculum_proposal_changes
ADD COLUMN unit_content TEXT,
ADD COLUMN previous_unit_content TEXT;

ALTER TABLE curriculum_proposal_changes
DISABLE TRIGGER curriculum_proposal_changes_accepted_immutable;

UPDATE curriculum_proposal_changes change
SET unit_content = COALESCE(
    (SELECT unit.content FROM units unit WHERE unit.id = change.unit_id),
    change.unit_description
)
WHERE kind IN ('create_unit', 'delete_unit');

ALTER TABLE curriculum_proposal_changes
ENABLE TRIGGER curriculum_proposal_changes_accepted_immutable;

ALTER TABLE curriculum_proposal_changes
DROP CONSTRAINT curriculum_proposal_changes_kind_check,
DROP CONSTRAINT curriculum_proposal_changes_check;

ALTER TABLE curriculum_proposal_changes
ADD CONSTRAINT curriculum_proposal_changes_kind_check
CHECK (kind IN (
    'create_unit', 'update_unit', 'update_content', 'delete_unit',
    'add_dependency', 'remove_dependency'
)),
ADD CONSTRAINT curriculum_proposal_changes_payload_check
CHECK (
    (
        kind = 'create_unit'
        AND unit_name IS NOT NULL AND unit_name <> ''
        AND unit_description IS NOT NULL AND unit_description <> ''
        AND unit_content IS NOT NULL AND unit_content <> ''
        AND previous_unit_name IS NULL
        AND previous_unit_description IS NULL
        AND previous_unit_content IS NULL
        AND prerequisite_id IS NULL
    )
    OR
    (
        kind = 'update_unit'
        AND unit_name IS NOT NULL AND unit_name <> ''
        AND unit_description IS NOT NULL AND unit_description <> ''
        AND previous_unit_name IS NOT NULL AND previous_unit_name <> ''
        AND previous_unit_description IS NOT NULL AND previous_unit_description <> ''
        AND unit_content IS NULL
        AND previous_unit_content IS NULL
        AND prerequisite_id IS NULL
    )
    OR
    (
        kind = 'update_content'
        AND unit_name IS NULL
        AND unit_description IS NULL
        AND previous_unit_name IS NULL
        AND previous_unit_description IS NULL
        AND unit_content IS NOT NULL AND unit_content <> ''
        AND previous_unit_content IS NOT NULL AND previous_unit_content <> ''
        AND prerequisite_id IS NULL
    )
    OR
    (
        kind = 'delete_unit'
        AND unit_name IS NOT NULL AND unit_name <> ''
        AND unit_description IS NOT NULL AND unit_description <> ''
        AND unit_content IS NOT NULL AND unit_content <> ''
        AND previous_unit_name IS NULL
        AND previous_unit_description IS NULL
        AND previous_unit_content IS NULL
        AND prerequisite_id IS NULL
    )
    OR
    (
        kind IN ('add_dependency', 'remove_dependency')
        AND unit_name IS NULL
        AND unit_description IS NULL
        AND previous_unit_name IS NULL
        AND previous_unit_description IS NULL
        AND unit_content IS NULL
        AND previous_unit_content IS NULL
        AND prerequisite_id IS NOT NULL
        AND prerequisite_id <> unit_id
    )
);

-- +goose Down
ALTER TABLE curriculum_proposal_changes
DROP CONSTRAINT curriculum_proposal_changes_payload_check,
DROP CONSTRAINT curriculum_proposal_changes_kind_check;

ALTER TABLE curriculum_proposal_changes
DISABLE TRIGGER curriculum_proposal_changes_accepted_immutable;

DELETE FROM curriculum_proposal_changes
WHERE kind = 'update_content';

ALTER TABLE curriculum_proposal_changes
ENABLE TRIGGER curriculum_proposal_changes_accepted_immutable;

ALTER TABLE curriculum_proposal_changes
DROP COLUMN previous_unit_content,
DROP COLUMN unit_content;

ALTER TABLE curriculum_proposal_changes
ADD CONSTRAINT curriculum_proposal_changes_kind_check
CHECK (kind IN (
    'create_unit', 'update_unit', 'delete_unit',
    'add_dependency', 'remove_dependency'
)),
ADD CONSTRAINT curriculum_proposal_changes_check
CHECK (
    (kind = 'create_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND prerequisite_id IS NULL)
    OR
    (kind = 'update_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND previous_unit_name IS NOT NULL AND previous_unit_name <> '' AND previous_unit_description IS NOT NULL AND previous_unit_description <> '' AND prerequisite_id IS NULL)
    OR
    (kind = 'delete_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND prerequisite_id IS NULL)
    OR
    (kind IN ('add_dependency', 'remove_dependency') AND unit_name IS NULL AND unit_description IS NULL AND previous_unit_name IS NULL AND previous_unit_description IS NULL AND prerequisite_id IS NOT NULL AND prerequisite_id <> unit_id)
);

ALTER TABLE units
DROP COLUMN content;
