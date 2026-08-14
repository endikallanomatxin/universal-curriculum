package db

import (
	"database/sql"
	"fmt"

	"universal-curriculum/internal/models"
)

func RebuildCurriculumProjection(q curriculumExecutor, proposalID int64) error {
	rows, err := q.Query(`
		WITH RECURSIVE proposal_lineage AS (
			SELECT id, base_proposal_id, 0 AS depth
			FROM curriculum_proposals
			WHERE id = $1 AND status = 'accepted'

			UNION ALL

			SELECT proposal.id, proposal.base_proposal_id, lineage.depth + 1
			FROM curriculum_proposals proposal
			JOIN proposal_lineage lineage ON proposal.id = lineage.base_proposal_id
			WHERE proposal.status = 'accepted'
		)
		SELECT change.kind, change.unit_id, COALESCE(change.unit_name, ''),
		       COALESCE(change.unit_content, ''),
		       change.prerequisite_id
		FROM proposal_lineage lineage
		JOIN curriculum_proposal_change_details change ON change.proposal_id = lineage.id
		ORDER BY lineage.depth DESC,
		         CASE change.kind
		             WHEN 'create_unit' THEN 1
		             WHEN 'rename_unit' THEN 2
		             WHEN 'update_content' THEN 2
		             WHEN 'remove_dependency' THEN 3
		             WHEN 'add_dependency' THEN 4
		             WHEN 'recognition' THEN 5
		             WHEN 'delete_unit' THEN 6
		         END,
		         change.unit_id NULLS LAST,
		         change.prerequisite_id NULLS LAST,
		         change.id
	`, proposalID)
	if err != nil {
		return fmt.Errorf("list accepted curriculum changes: %w", err)
	}
	defer rows.Close()
	var changes []models.CurriculumProposalChange
	for rows.Next() {
		var change models.CurriculumProposalChange
		var unitID, prerequisite sql.NullInt64
		if err := rows.Scan(
			&change.Kind, &unitID, &change.UnitName,
			&change.UnitContent, &prerequisite,
		); err != nil {
			return fmt.Errorf("scan accepted curriculum change: %w", err)
		}
		if unitID.Valid {
			change.UnitID = unitID.Int64
		}
		if prerequisite.Valid {
			change.PrerequisiteID = &prerequisite.Int64
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate accepted curriculum changes: %w", err)
	}
	if _, err := q.Exec(`
		CREATE TEMP TABLE curriculum_rebuild_units (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			content TEXT NOT NULL
		) ON COMMIT DROP;
		CREATE TEMP TABLE curriculum_rebuild_dependencies (
			unit_id BIGINT NOT NULL,
			prerequisite_id BIGINT NOT NULL,
			PRIMARY KEY (unit_id, prerequisite_id)
		) ON COMMIT DROP
	`); err != nil {
		return fmt.Errorf("prepare curriculum projection rebuild: %w", err)
	}
	for _, change := range changes {
		switch change.Kind {
		case "create_unit":
			if _, err := q.Exec(`
				INSERT INTO curriculum_rebuild_units (id, name, content)
				VALUES ($1, $2, $3)
			`, change.UnitID, change.UnitName, change.UnitContent); err != nil {
				return fmt.Errorf("project unit creation: %w", err)
			}
		case "delete_unit":
			if _, err := q.Exec(`DELETE FROM curriculum_rebuild_units WHERE id = $1`, change.UnitID); err != nil {
				return fmt.Errorf("project unit deletion: %w", err)
			}
			if _, err := q.Exec(`
				DELETE FROM curriculum_rebuild_dependencies
				WHERE unit_id = $1 OR prerequisite_id = $1
			`, change.UnitID); err != nil {
				return fmt.Errorf("project deleted unit dependencies: %w", err)
			}
		case "rename_unit":
			if _, err := q.Exec(`
				UPDATE curriculum_rebuild_units SET name = $2 WHERE id = $1
			`, change.UnitID, change.UnitName); err != nil {
				return fmt.Errorf("project unit update: %w", err)
			}
		case "update_content":
			if _, err := q.Exec(`
				UPDATE curriculum_rebuild_units SET content = $2 WHERE id = $1
			`, change.UnitID, change.UnitContent); err != nil {
				return fmt.Errorf("project unit content update: %w", err)
			}
		case "add_dependency":
			if _, err := q.Exec(`
				INSERT INTO curriculum_rebuild_dependencies (unit_id, prerequisite_id)
				VALUES ($1, $2)
			`, change.UnitID, change.PrerequisiteID); err != nil {
				return fmt.Errorf("project dependency creation: %w", err)
			}
		case "remove_dependency":
			if _, err := q.Exec(`
				DELETE FROM curriculum_rebuild_dependencies
				WHERE unit_id = $1 AND prerequisite_id = $2
			`, change.UnitID, change.PrerequisiteID); err != nil {
				return fmt.Errorf("project dependency deletion: %w", err)
			}
		case "recognition":
			// Recognitions describe continuity between curriculum states;
			// they do not change the published graph projection.
		default:
			return fmt.Errorf("project unsupported curriculum change %q", change.Kind)
		}
	}
	if _, err := q.Exec(`DELETE FROM unit_dependencies`); err != nil {
		return fmt.Errorf("clear projected curriculum dependencies: %w", err)
	}
	if _, err := q.Exec(`
		INSERT INTO units (id, name, content)
		SELECT id, name, content
		FROM curriculum_rebuild_units
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
		    content = EXCLUDED.content,
		    updated_at = CASE
		        WHEN (units.name, units.content)
		             IS DISTINCT FROM (EXCLUDED.name, EXCLUDED.content)
		        THEN NOW()
		        ELSE units.updated_at
		    END
	`); err != nil {
		return fmt.Errorf("reconcile projected curriculum units: %w", err)
	}
	if _, err := q.Exec(`
		DELETE FROM units unit
		WHERE NOT EXISTS (
			SELECT 1 FROM curriculum_rebuild_units rebuilt WHERE rebuilt.id = unit.id
		)
	`); err != nil {
		return fmt.Errorf("remove deleted projected curriculum units: %w", err)
	}
	if _, err := q.Exec(`
		INSERT INTO unit_dependencies (unit_id, prerequisite_id)
		SELECT unit_id, prerequisite_id
		FROM curriculum_rebuild_dependencies
	`); err != nil {
		return fmt.Errorf("reconcile projected curriculum dependencies: %w", err)
	}
	result, err := q.Exec(`
		UPDATE curriculum_projection_state
		SET proposal_id = $1
		WHERE singleton = TRUE
	`, proposalID)
	if err != nil {
		return fmt.Errorf("update curriculum projection: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return fmt.Errorf("update curriculum projection: affected %d rows: %w", updated, err)
	}
	return nil
}
