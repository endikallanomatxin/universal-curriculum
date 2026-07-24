package db

import (
	"database/sql"
	"errors"
	"fmt"

	"universal-curriculum/internal/models"
)

func CurrentCurriculumVersion(q curriculumExecutor) (int64, error) {
	var version int64
	if err := q.QueryRow(`SELECT version FROM curriculum_projection_state WHERE singleton = TRUE FOR UPDATE`).Scan(&version); err != nil {
		return 0, fmt.Errorf("lock curriculum projection version: %w", err)
	}
	return version, nil
}

func NextCurriculumUnitID(q curriculumExecutor) (int64, error) {
	var id int64
	if err := q.QueryRow(`SELECT nextval('curriculum_unit_ids')`).Scan(&id); err != nil {
		return 0, fmt.Errorf("allocate curriculum unit id: %w", err)
	}
	return id, nil
}

func CreateDraftCurriculumProposal(q curriculumExecutor, proposal *models.CurriculumProposal) error {
	err := q.QueryRow(`
		INSERT INTO curriculum_proposals (author_id, title, rationale, status, base_version)
		VALUES ($1, $2, $3, 'draft', $4)
		RETURNING id, created_at
	`, proposal.AuthorID, proposal.Title, proposal.Rationale, proposal.BaseVersion).
		Scan(&proposal.ID, &proposal.CreatedAt)
	if err != nil {
		return fmt.Errorf("create draft curriculum proposal: %w", err)
	}
	proposal.Status = "draft"
	return nil
}

func GetCurriculumProposal(q curriculumExecutor, proposalID int64) (*models.CurriculumProposal, error) {
	var proposal models.CurriculumProposal
	var authorID, version, reverts sql.NullInt64
	var acceptedAt sql.NullTime
	err := q.QueryRow(`
		SELECT proposal.id, proposal.author_id, COALESCE(author.full_name, 'System'),
		       proposal.title, proposal.rationale, proposal.status, proposal.base_version,
		       proposal.published_version, proposal.reverts_proposal_id,
		       proposal.created_at, proposal.accepted_at
		FROM curriculum_proposals proposal
		LEFT JOIN users author ON author.id = proposal.author_id
		WHERE proposal.id = $1
	`, proposalID).Scan(
		&proposal.ID, &authorID, &proposal.AuthorName, &proposal.Title,
		&proposal.Rationale, &proposal.Status, &proposal.BaseVersion, &version,
		&reverts, &proposal.CreatedAt, &acceptedAt,
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
	if version.Valid {
		proposal.PublishedVersion = &version.Int64
	}
	if reverts.Valid {
		proposal.RevertsProposalID = &reverts.Int64
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
		INSERT INTO curriculum_proposal_changes (
			proposal_id, position, kind, unit_id, unit_name, previous_unit_name,
			unit_content, previous_unit_content, prerequisite_id
		)
		SELECT proposal.id,
		       COALESCE((SELECT MAX(position) + 1 FROM curriculum_proposal_changes WHERE proposal_id = proposal.id), 1),
		       $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''),
		       NULLIF($8, ''), $9
		FROM curriculum_proposals proposal
		WHERE proposal.id = $1 AND proposal.author_id = $2 AND proposal.status = 'draft'
		RETURNING id, position
	`, proposalID, authorID, change.Kind, change.UnitID, change.UnitName,
		change.PreviousUnitName, change.UnitContent, change.PreviousUnitContent, change.PrerequisiteID,
	).Scan(&change.ID, &change.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return fmt.Errorf("add draft curriculum proposal change: %w", err)
	}
	return nil
}

func UpdateDraftCreatedCurriculumUnit(q curriculumExecutor, proposalID, authorID, unitID int64, name, content string) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposal_changes change
		SET unit_name = $4, unit_content = $5
		FROM curriculum_proposals proposal
		WHERE change.proposal_id = $1
		  AND change.unit_id = $3
		  AND change.kind = 'create_unit'
		  AND proposal.id = change.proposal_id
		  AND proposal.author_id = $2
		  AND proposal.status = 'draft'
	`, proposalID, authorID, unitID, name, content)
	if err != nil {
		return false, fmt.Errorf("update draft created curriculum unit: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
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
			USING authorized_proposal proposal
			WHERE change.proposal_id = proposal.id
			  AND change.unit_id = $3
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

func CreateAcceptedCurriculumProposal(q curriculumExecutor, proposal *models.CurriculumProposal) error {
	err := q.QueryRow(`
		INSERT INTO curriculum_proposals (
			author_id, title, rationale, status, base_version, reverts_proposal_id
		)
		VALUES ($1, $2, $3, 'draft', $4, $5)
		RETURNING id, created_at
	`, proposal.AuthorID, proposal.Title, proposal.Rationale, proposal.BaseVersion,
		proposal.RevertsProposalID,
	).Scan(&proposal.ID, &proposal.CreatedAt)
	if err != nil {
		return fmt.Errorf("create accepted curriculum proposal: %w", err)
	}
	for index := range proposal.Changes {
		change := &proposal.Changes[index]
		change.ProposalID = proposal.ID
		change.Position = index + 1
		err := q.QueryRow(`
			INSERT INTO curriculum_proposal_changes (
				proposal_id, position, kind, unit_id, unit_name, previous_unit_name,
				unit_content, previous_unit_content, prerequisite_id
			)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''),
			        NULLIF($7, ''), NULLIF($8, ''), $9)
			RETURNING id
		`, change.ProposalID, change.Position, change.Kind, change.UnitID,
			change.UnitName, change.PreviousUnitName, change.UnitContent,
			change.PreviousUnitContent, change.PrerequisiteID,
		).Scan(&change.ID)
		if err != nil {
			return fmt.Errorf("create curriculum proposal change: %w", err)
		}
	}
	if err := q.QueryRow(`
		UPDATE curriculum_proposals
		SET status = 'accepted', published_version = $2, accepted_at = NOW()
		WHERE id = $1
		RETURNING accepted_at
	`, proposal.ID, proposal.PublishedVersion).Scan(&proposal.AcceptedAt); err != nil {
		return fmt.Errorf("accept curriculum proposal: %w", err)
	}
	return nil
}

func AcceptDraftCurriculumProposal(q curriculumExecutor, proposalID, authorID, publishedVersion int64) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals
		SET status = 'accepted', published_version = $3, accepted_at = NOW()
		WHERE id = $1 AND author_id = $2 AND status = 'draft'
	`, proposalID, authorID, publishedVersion)
	if err != nil {
		return false, fmt.Errorf("accept draft curriculum proposal: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func RebuildCurriculumProjection(q curriculumExecutor, version, proposalID int64) error {
	rows, err := q.Query(`
		SELECT change.kind, change.unit_id, COALESCE(change.unit_name, ''),
		       COALESCE(change.unit_content, ''),
		       change.prerequisite_id
		FROM curriculum_proposals proposal
		JOIN curriculum_proposal_changes change ON change.proposal_id = proposal.id
		WHERE proposal.status = 'accepted'
		ORDER BY proposal.published_version, change.position
	`)
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
		case "update_unit":
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
		SET version = $1, proposal_id = $2
		WHERE singleton = TRUE
	`, version, proposalID)
	if err != nil {
		return fmt.Errorf("update curriculum projection version: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return fmt.Errorf("update curriculum projection version: affected %d rows: %w", updated, err)
	}
	return nil
}

func ListCurriculumProposals(database *sql.DB, limit int) ([]models.CurriculumProposal, error) {
	rows, err := database.Query(`
		SELECT proposal.id, proposal.author_id, COALESCE(author.full_name, 'System'),
		       proposal.title, proposal.rationale, proposal.status,
		       proposal.base_version, proposal.published_version,
		       proposal.reverts_proposal_id, proposal.created_at, proposal.accepted_at
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
		var authorID, version, reverts sql.NullInt64
		var acceptedAt sql.NullTime
		if err := rows.Scan(
			&proposal.ID, &authorID, &proposal.AuthorName, &proposal.Title,
			&proposal.Rationale, &proposal.Status, &proposal.BaseVersion,
			&version, &reverts, &proposal.CreatedAt, &acceptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan curriculum proposal: %w", err)
		}
		if authorID.Valid {
			proposal.AuthorID = &authorID.Int64
		}
		if version.Valid {
			proposal.PublishedVersion = &version.Int64
		}
		if reverts.Valid {
			proposal.RevertsProposalID = &reverts.Int64
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
		       proposal.base_version, proposal.created_at,
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
		var proposalAuthorID sql.NullInt64
		if err := rows.Scan(
			&proposal.ID, &proposalAuthorID, &proposal.AuthorName, &proposal.Title,
			&proposal.Rationale, &proposal.Status, &proposal.BaseVersion,
			&proposal.CreatedAt, &proposal.ChangeCount,
		); err != nil {
			return nil, fmt.Errorf("scan draft curriculum proposal: %w", err)
		}
		if proposalAuthorID.Valid {
			proposal.AuthorID = &proposalAuthorID.Int64
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft curriculum proposals: %w", err)
	}
	return proposals, nil
}

func GetLatestRevertibleCurriculumProposal(q curriculumExecutor) (*models.CurriculumProposal, error) {
	var proposal models.CurriculumProposal
	var version int64
	err := q.QueryRow(`
		SELECT id, title, published_version
		FROM curriculum_proposals
		WHERE status = 'accepted' AND author_id IS NOT NULL
		ORDER BY published_version DESC
		LIMIT 1
		FOR UPDATE
	`).Scan(&proposal.ID, &proposal.Title, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest revertible curriculum proposal: %w", err)
	}
	proposal.PublishedVersion = &version
	proposal.Changes, err = listCurriculumProposalChanges(q, proposal.ID)
	return &proposal, err
}

func listCurriculumProposalChanges(q curriculumExecutor, proposalID int64) ([]models.CurriculumProposalChange, error) {
	rows, err := q.Query(`
		SELECT id, proposal_id, position, kind, unit_id,
		       COALESCE(unit_name, ''), COALESCE(previous_unit_name, ''),
		       COALESCE(unit_content, ''), COALESCE(previous_unit_content, ''),
		       prerequisite_id
		FROM curriculum_proposal_changes
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
		if err := rows.Scan(
			&change.ID, &change.ProposalID, &change.Position, &change.Kind,
			&change.UnitID, &change.UnitName, &change.PreviousUnitName,
			&change.UnitContent, &change.PreviousUnitContent, &prerequisite,
		); err != nil {
			return nil, fmt.Errorf("scan curriculum proposal change: %w", err)
		}
		if prerequisite.Valid {
			change.PrerequisiteID = &prerequisite.Int64
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}
