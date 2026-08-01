package db

import (
	"database/sql"
	"errors"
	"fmt"

	"universal-curriculum/internal/models"
)

func LockCurrentCurriculumProposal(q curriculumExecutor) (*int64, error) {
	var proposalID sql.NullInt64
	if err := q.QueryRow(`
		SELECT proposal_id
		FROM curriculum_projection_state
		WHERE singleton = TRUE
		FOR UPDATE
	`).Scan(&proposalID); err != nil {
		return nil, fmt.Errorf("lock curriculum projection: %w", err)
	}
	if !proposalID.Valid {
		return nil, nil
	}
	return &proposalID.Int64, nil
}

func CreateDraftCurriculumProposal(q curriculumExecutor, proposal *models.CurriculumProposal) error {
	err := q.QueryRow(`
		INSERT INTO curriculum_proposals (author_id, title, rationale, status, base_proposal_id)
		VALUES ($1, $2, $3, 'draft', $4)
		RETURNING id, created_at
	`, proposal.AuthorID, proposal.Title, proposal.Rationale, proposal.BaseProposalID).
		Scan(&proposal.ID, &proposal.CreatedAt)
	if err != nil {
		return fmt.Errorf("create draft curriculum proposal: %w", err)
	}
	proposal.Status = "draft"
	return nil
}

func GetCurriculumProposal(q curriculumExecutor, proposalID int64) (*models.CurriculumProposal, error) {
	var proposal models.CurriculumProposal
	var authorID, baseProposalID sql.NullInt64
	var acceptedAt sql.NullTime
	err := q.QueryRow(`
		SELECT proposal.id, proposal.author_id, COALESCE(author.full_name, 'System'),
		       proposal.title, proposal.rationale, proposal.status, proposal.base_proposal_id,
		       proposal.created_at, proposal.accepted_at
		FROM curriculum_proposals proposal
		LEFT JOIN users author ON author.id = proposal.author_id
		WHERE proposal.id = $1
	`, proposalID).Scan(
		&proposal.ID, &authorID, &proposal.AuthorName, &proposal.Title,
		&proposal.Rationale, &proposal.Status, &baseProposalID,
		&proposal.CreatedAt, &acceptedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get curriculum proposal: %w", err)
	}
	if authorID.Valid {
		proposal.AuthorID = &authorID.Int64
	}
	if baseProposalID.Valid {
		proposal.BaseProposalID = &baseProposalID.Int64
	}
	if acceptedAt.Valid {
		proposal.AcceptedAt = &acceptedAt.Time
	}
	changes, err := listCurriculumProposalChanges(q, proposal.ID)
	if err != nil {
		return nil, err
	}
	proposal.Changes = changes
	return &proposal, nil
}

func UpdateDraftCurriculumProposal(q curriculumExecutor, proposalID, authorID int64, title, rationale string) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals SET title = $3, rationale = $4
		WHERE id = $1 AND author_id = $2 AND status = 'draft'
	`, proposalID, authorID, title, rationale)
	if err != nil {
		return false, fmt.Errorf("update draft curriculum proposal: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func DeleteDraftCurriculumProposal(q curriculumExecutor, proposalID, authorID int64) (bool, error) {
	result, err := q.Exec(`
		DELETE FROM curriculum_proposals
		WHERE id = $1 AND author_id = $2 AND status = 'draft'
	`, proposalID, authorID)
	if err != nil {
		return false, fmt.Errorf("delete draft curriculum proposal: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func AddDraftCurriculumProposalChange(q curriculumExecutor, proposalID, authorID int64, change *models.CurriculumProposalChange) error {
	change.ProposalID = proposalID
	err := q.QueryRow(`
		INSERT INTO curriculum_proposal_changes (proposal_id, position, kind)
		SELECT proposal.id,
		       COALESCE((SELECT MAX(position) + 1 FROM curriculum_proposal_changes WHERE proposal_id = proposal.id), 1),
		       $3
		FROM curriculum_proposals proposal
		WHERE proposal.id = $1 AND proposal.author_id = $2 AND proposal.status = 'draft'
		RETURNING id, position
	`, proposalID, authorID, change.Kind).Scan(&change.ID, &change.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return fmt.Errorf("add draft curriculum proposal change: %w", err)
	}
	return insertCurriculumProposalChangeDetail(q, change)
}

func DeleteDraftCurriculumProposalChange(q curriculumExecutor, proposalID, changeID, authorID int64) (bool, error) {
	result, err := q.Exec(`
		DELETE FROM curriculum_proposal_changes change
		USING curriculum_proposals proposal
		WHERE change.id = $2 AND change.proposal_id = $1
		  AND proposal.id = change.proposal_id AND proposal.author_id = $3
		  AND proposal.status = 'draft'
	`, proposalID, changeID, authorID)
	if err != nil {
		return false, fmt.Errorf("delete draft curriculum proposal change: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func DeleteDraftCurriculumProposalUnitChanges(q curriculumExecutor, proposalID, authorID, unitID int64, kind string) (bool, error) {
	var authorized bool
	err := q.QueryRow(`
		WITH authorized_proposal AS (
			SELECT id
			FROM curriculum_proposals
			WHERE id = $1 AND author_id = $2 AND status = 'draft'
		),
		deleted AS (
			DELETE FROM curriculum_proposal_changes change
			USING authorized_proposal proposal, curriculum_proposal_change_details detail
			WHERE change.proposal_id = proposal.id
			  AND detail.id = change.id
			  AND detail.unit_id = $3
			  AND change.kind = $4
			RETURNING change.id
		)
		SELECT EXISTS (SELECT 1 FROM authorized_proposal)
	`, proposalID, authorID, unitID, kind).Scan(&authorized)
	if err != nil {
		return false, fmt.Errorf("replace draft curriculum unit change: %w", err)
	}
	return authorized, nil
}

func AcceptDraftCurriculumProposal(q curriculumExecutor, proposalID, authorID int64) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals
		SET status = 'accepted', accepted_at = clock_timestamp()
		WHERE id = $1 AND author_id = $2 AND status = 'draft'
	`, proposalID, authorID)
	if err != nil {
		return false, fmt.Errorf("accept draft curriculum proposal: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func ListDraftCurriculumProposalIDs(q curriculumExecutor) ([]int64, error) {
	rows, err := q.Query(`
		SELECT id
		FROM curriculum_proposals
		WHERE status = 'draft'
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list draft curriculum proposal ids: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan draft curriculum proposal id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft curriculum proposal ids: %w", err)
	}
	return ids, nil
}

func SetDraftCurriculumProposalBase(
	q curriculumExecutor,
	proposalID int64,
	expectedBaseProposalID, baseProposalID *int64,
) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals
		SET base_proposal_id = $3
		WHERE id = $1
		  AND status = 'draft'
		  AND base_proposal_id IS NOT DISTINCT FROM $2
	`, proposalID, expectedBaseProposalID, baseProposalID)
	if err != nil {
		return false, fmt.Errorf("set draft curriculum proposal base: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func UpdateDraftCurriculumProposalChangeForRebase(
	q curriculumExecutor,
	change models.CurriculumProposalChange,
) error {
	var result sql.Result
	var err error
	switch change.Kind {
	case "rename_unit":
		result, err = q.Exec(`
			UPDATE curriculum_unit_renames rename
			SET previous_name = $2
			FROM curriculum_proposal_changes change, curriculum_proposals proposal
			WHERE rename.change_id = $1
			  AND change.id = rename.change_id
			  AND proposal.id = change.proposal_id
			  AND proposal.status = 'draft'
		`, change.ID, change.PreviousUnitName)
	case "update_content":
		result, err = q.Exec(`
			UPDATE curriculum_unit_content_updates content_update
			SET content = $2, previous_content = $3
			FROM curriculum_proposal_changes change, curriculum_proposals proposal
			WHERE content_update.change_id = $1
			  AND change.id = content_update.change_id
			  AND proposal.id = change.proposal_id
			  AND proposal.status = 'draft'
		`, change.ID, change.UnitContent, change.PreviousUnitContent)
	case "delete_unit":
		result, err = q.Exec(`
			UPDATE curriculum_unit_deletions deletion
			SET name = $2, content = $3
			FROM curriculum_proposal_changes change, curriculum_proposals proposal
			WHERE deletion.change_id = $1
			  AND change.id = deletion.change_id
			  AND proposal.id = change.proposal_id
			  AND proposal.status = 'draft'
		`, change.ID, change.UnitName, change.UnitContent)
	default:
		return nil
	}
	if err != nil {
		return fmt.Errorf("update draft curriculum proposal change for rebase: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read draft curriculum proposal change snapshot result: %w", err)
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

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
		ORDER BY lineage.depth DESC, change.position
	`, proposalID)
	if err != nil {
		return fmt.Errorf("list accepted curriculum changes: %w", err)
	}
	defer rows.Close()
	var changes []models.CurriculumProposalChange
	for rows.Next() {
		var change models.CurriculumProposalChange
		var prerequisite sql.NullInt64
		if err := rows.Scan(
			&change.Kind, &change.UnitID, &change.UnitName,
			&change.UnitContent, &prerequisite,
		); err != nil {
			return fmt.Errorf("scan accepted curriculum change: %w", err)
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

func ListCurriculumProposals(database *sql.DB, limit int) ([]models.CurriculumProposal, error) {
	rows, err := database.Query(`
		SELECT proposal.id, proposal.author_id, COALESCE(author.full_name, 'System'),
		       proposal.title, proposal.rationale, proposal.status,
		       proposal.base_proposal_id,
		       proposal.created_at, proposal.accepted_at
		FROM curriculum_proposals proposal
		LEFT JOIN users author ON author.id = proposal.author_id
		WHERE proposal.status <> 'draft'
		ORDER BY proposal.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list curriculum proposals: %w", err)
	}
	defer rows.Close()
	var proposals []models.CurriculumProposal
	for rows.Next() {
		var proposal models.CurriculumProposal
		var authorID, baseProposalID sql.NullInt64
		var acceptedAt sql.NullTime
		if err := rows.Scan(
			&proposal.ID, &authorID, &proposal.AuthorName, &proposal.Title,
			&proposal.Rationale, &proposal.Status, &baseProposalID,
			&proposal.CreatedAt, &acceptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan curriculum proposal: %w", err)
		}
		if authorID.Valid {
			proposal.AuthorID = &authorID.Int64
		}
		if baseProposalID.Valid {
			proposal.BaseProposalID = &baseProposalID.Int64
		}
		if acceptedAt.Valid {
			proposal.AcceptedAt = &acceptedAt.Time
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate curriculum proposals: %w", err)
	}
	return proposals, nil
}

func ListDraftCurriculumProposalsByAuthor(database *sql.DB, authorID int64) ([]models.CurriculumProposal, error) {
	rows, err := database.Query(`
		SELECT proposal.id, proposal.author_id, COALESCE(author.full_name, 'System'),
		       proposal.title, proposal.rationale, proposal.status,
		       proposal.base_proposal_id, proposal.created_at,
		       COUNT(change.id)
		FROM curriculum_proposals proposal
		LEFT JOIN users author ON author.id = proposal.author_id
		LEFT JOIN curriculum_proposal_changes change ON change.proposal_id = proposal.id
		WHERE proposal.status = 'draft' AND proposal.author_id = $1
		GROUP BY proposal.id, author.full_name
		ORDER BY proposal.created_at DESC
	`, authorID)
	if err != nil {
		return nil, fmt.Errorf("list draft curriculum proposals by author: %w", err)
	}
	defer rows.Close()
	var proposals []models.CurriculumProposal
	for rows.Next() {
		var proposal models.CurriculumProposal
		var proposalAuthorID, baseProposalID sql.NullInt64
		if err := rows.Scan(
			&proposal.ID, &proposalAuthorID, &proposal.AuthorName, &proposal.Title,
			&proposal.Rationale, &proposal.Status, &baseProposalID,
			&proposal.CreatedAt, &proposal.ChangeCount,
		); err != nil {
			return nil, fmt.Errorf("scan draft curriculum proposal: %w", err)
		}
		if proposalAuthorID.Valid {
			proposal.AuthorID = &proposalAuthorID.Int64
		}
		if baseProposalID.Valid {
			proposal.BaseProposalID = &baseProposalID.Int64
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft curriculum proposals: %w", err)
	}
	return proposals, nil
}

func listCurriculumProposalChanges(q curriculumExecutor, proposalID int64) ([]models.CurriculumProposalChange, error) {
	rows, err := q.Query(`
		SELECT id, proposal_id, position, kind, unit_id,
		       COALESCE(unit_name, ''), COALESCE(previous_unit_name, ''),
		       COALESCE(unit_content, ''), COALESCE(previous_unit_content, ''),
		       prerequisite_id, COALESCE(recognition_rationale, '')
		FROM curriculum_proposal_change_details
		WHERE proposal_id = $1
		ORDER BY position
	`, proposalID)
	if err != nil {
		return nil, fmt.Errorf("list curriculum proposal changes: %w", err)
	}
	defer rows.Close()
	var changes []models.CurriculumProposalChange
	for rows.Next() {
		var change models.CurriculumProposalChange
		var prerequisite sql.NullInt64
		var recognitionRationale string
		if err := rows.Scan(
			&change.ID, &change.ProposalID, &change.Position, &change.Kind,
			&change.UnitID, &change.UnitName, &change.PreviousUnitName,
			&change.UnitContent, &change.PreviousUnitContent, &prerequisite,
			&recognitionRationale,
		); err != nil {
			return nil, fmt.Errorf("scan curriculum proposal change: %w", err)
		}
		if prerequisite.Valid {
			change.PrerequisiteID = &prerequisite.Int64
		}
		if change.Kind == "recognition" {
			change.Recognition = &models.Recognition{Rationale: recognitionRationale}
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close curriculum proposal changes: %w", err)
	}
	if err := loadCurriculumRecognitionMembers(q, proposalID, changes); err != nil {
		return nil, err
	}
	return changes, nil
}

func insertCurriculumProposalChangeDetail(q curriculumExecutor, change *models.CurriculumProposalChange) error {
	var err error
	switch change.Kind {
	case "create_unit":
		change.UnitID = change.ID
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_creations (change_id, name, content)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitName, change.UnitContent)
	case "rename_unit":
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_renames (change_id, unit_id, name, previous_name)
			VALUES ($1, $2, $3, $4)
		`, change.ID, change.UnitID, change.UnitName, change.PreviousUnitName)
	case "update_content":
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_content_updates (
				change_id, unit_id, content, previous_content
			)
			VALUES ($1, $2, $3, $4)
		`, change.ID, change.UnitID, change.UnitContent, change.PreviousUnitContent)
	case "delete_unit":
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_deletions (change_id, unit_id, name, content)
			VALUES ($1, $2, $3, $4)
		`, change.ID, change.UnitID, change.UnitName, change.UnitContent)
	case "add_dependency":
		_, err = q.Exec(`
			INSERT INTO curriculum_dependency_additions (
				change_id, unit_id, prerequisite_id
			)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitID, change.PrerequisiteID)
	case "remove_dependency":
		_, err = q.Exec(`
			INSERT INTO curriculum_dependency_removals (
				change_id, unit_id, prerequisite_id
			)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitID, change.PrerequisiteID)
	case "recognition":
		if change.Recognition == nil {
			return fmt.Errorf("create curriculum recognition detail: missing recognition")
		}
		_, err = q.Exec(`
			INSERT INTO curriculum_recognitions (change_id, rationale)
			VALUES ($1, $2)
		`, change.ID, change.Recognition.Rationale)
		if err == nil {
			for _, source := range change.Recognition.Sources {
				if _, err = q.Exec(`
					INSERT INTO curriculum_recognition_sources (recognition_change_id, unit_id)
					VALUES ($1, $2)
				`, change.ID, source.ID); err != nil {
					break
				}
			}
		}
		if err == nil {
			for _, target := range change.Recognition.Targets {
				if _, err = q.Exec(`
					INSERT INTO curriculum_recognition_targets (recognition_change_id, unit_id)
					VALUES ($1, $2)
				`, change.ID, target.ID); err != nil {
					break
				}
			}
		}
	default:
		return fmt.Errorf("create unsupported curriculum proposal change %q", change.Kind)
	}
	if err != nil {
		return fmt.Errorf("create curriculum proposal %s detail: %w", change.Kind, err)
	}
	return nil
}

func loadCurriculumRecognitionMembers(
	q curriculumExecutor,
	proposalID int64,
	changes []models.CurriculumProposalChange,
) error {
	changeIndexes := make(map[int64]int)
	for index := range changes {
		if changes[index].Recognition != nil {
			changeIndexes[changes[index].ID] = index
		}
	}
	if len(changeIndexes) == 0 {
		return nil
	}
	rows, err := q.Query(`
		SELECT member.recognition_change_id, member.role, member.unit_id,
		       COALESCE(unit.name, creation.name)
		FROM (
			SELECT source.recognition_change_id, 'source'::TEXT AS role, source.unit_id
			FROM curriculum_recognition_sources source
			UNION ALL
			SELECT target.recognition_change_id, 'target'::TEXT AS role, target.unit_id
			FROM curriculum_recognition_targets target
		) member
		JOIN curriculum_unit_creations creation ON creation.change_id = member.unit_id
		JOIN curriculum_recognitions recognition ON recognition.change_id = member.recognition_change_id
		JOIN curriculum_proposal_changes change ON change.id = recognition.change_id
		LEFT JOIN units unit ON unit.id = member.unit_id
		WHERE change.proposal_id = $1
		ORDER BY member.recognition_change_id, member.role, lower(COALESCE(unit.name, creation.name)), member.unit_id
	`, proposalID)
	if err != nil {
		return fmt.Errorf("list curriculum recognition members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var changeID, unitID int64
		var role, name string
		if err := rows.Scan(&changeID, &role, &unitID, &name); err != nil {
			return fmt.Errorf("scan curriculum recognition member: %w", err)
		}
		index, exists := changeIndexes[changeID]
		if !exists {
			return fmt.Errorf("recognition member references unknown proposal change %d", changeID)
		}
		unit := models.Unit{ID: unitID, Name: name}
		if role == "source" {
			changes[index].Recognition.Sources = append(
				changes[index].Recognition.Sources, unit,
			)
		} else {
			changes[index].Recognition.Targets = append(
				changes[index].Recognition.Targets, unit,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate curriculum recognition members: %w", err)
	}
	return nil
}
