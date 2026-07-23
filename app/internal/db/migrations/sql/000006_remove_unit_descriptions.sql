-- +goose Up
ALTER TABLE curriculum_proposal_changes
DROP CONSTRAINT curriculum_proposal_changes_payload_check;

ALTER TABLE curriculum_proposal_changes
DROP COLUMN unit_description,
DROP COLUMN previous_unit_description;

ALTER TABLE curriculum_proposal_changes
ADD CONSTRAINT curriculum_proposal_changes_payload_check
CHECK (
    (kind = 'create_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_content IS NOT NULL AND unit_content <> '' AND previous_unit_name IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NULL)
    OR
    (kind = 'update_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND previous_unit_name IS NOT NULL AND previous_unit_name <> '' AND unit_content IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NULL)
    OR
    (kind = 'update_content' AND unit_name IS NULL AND previous_unit_name IS NULL AND unit_content IS NOT NULL AND unit_content <> '' AND previous_unit_content IS NOT NULL AND previous_unit_content <> '' AND prerequisite_id IS NULL)
    OR
    (kind = 'delete_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_content IS NOT NULL AND unit_content <> '' AND previous_unit_name IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NULL)
    OR
    (kind IN ('add_dependency', 'remove_dependency') AND unit_name IS NULL AND previous_unit_name IS NULL AND unit_content IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NOT NULL AND prerequisite_id <> unit_id)
);

ALTER TABLE units
DROP COLUMN description;

-- +goose Down
ALTER TABLE units
ADD COLUMN description TEXT;

UPDATE units
SET description = name;

ALTER TABLE units
ALTER COLUMN description SET NOT NULL,
ADD CONSTRAINT units_description_check
CHECK (description <> '' AND description = btrim(description));

ALTER TABLE curriculum_proposal_changes
DROP CONSTRAINT curriculum_proposal_changes_payload_check;

ALTER TABLE curriculum_proposal_changes
ADD COLUMN unit_description TEXT,
ADD COLUMN previous_unit_description TEXT;

ALTER TABLE curriculum_proposal_changes
DISABLE TRIGGER curriculum_proposal_changes_accepted_immutable;

UPDATE curriculum_proposal_changes
SET unit_description = unit_name
WHERE kind IN ('create_unit', 'update_unit', 'delete_unit');

UPDATE curriculum_proposal_changes
SET previous_unit_description = previous_unit_name
WHERE kind = 'update_unit';

ALTER TABLE curriculum_proposal_changes
ENABLE TRIGGER curriculum_proposal_changes_accepted_immutable;

ALTER TABLE curriculum_proposal_changes
ADD CONSTRAINT curriculum_proposal_changes_payload_check
CHECK (
    (kind = 'create_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND unit_content IS NOT NULL AND unit_content <> '' AND previous_unit_name IS NULL AND previous_unit_description IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NULL)
    OR
    (kind = 'update_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND previous_unit_name IS NOT NULL AND previous_unit_name <> '' AND previous_unit_description IS NOT NULL AND previous_unit_description <> '' AND unit_content IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NULL)
    OR
    (kind = 'update_content' AND unit_name IS NULL AND unit_description IS NULL AND previous_unit_name IS NULL AND previous_unit_description IS NULL AND unit_content IS NOT NULL AND unit_content <> '' AND previous_unit_content IS NOT NULL AND previous_unit_content <> '' AND prerequisite_id IS NULL)
    OR
    (kind = 'delete_unit' AND unit_name IS NOT NULL AND unit_name <> '' AND unit_description IS NOT NULL AND unit_description <> '' AND unit_content IS NOT NULL AND unit_content <> '' AND previous_unit_name IS NULL AND previous_unit_description IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NULL)
    OR
    (kind IN ('add_dependency', 'remove_dependency') AND unit_name IS NULL AND unit_description IS NULL AND previous_unit_name IS NULL AND previous_unit_description IS NULL AND unit_content IS NULL AND previous_unit_content IS NULL AND prerequisite_id IS NOT NULL AND prerequisite_id <> unit_id)
);
