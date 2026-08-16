package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"

	"universal-curriculum/internal/models"
)

func LockCurrentCurriculumProposal(q curriculumExecutor) (*int64, error) {
	return currentCurriculumProposalID(q, "FOR UPDATE", "lock curriculum projection")
}

func LockCurrentCurriculumProposalShared(q curriculumExecutor) (*int64, error) {
	return currentCurriculumProposalID(q, "FOR SHARE", "share curriculum projection")
}

func GetCurrentCurriculumProposalID(q curriculumExecutor) (*int64, error) {
	return currentCurriculumProposalID(q, "", "get curriculum projection")
}

func currentCurriculumProposalID(q curriculumExecutor, lock, operation string) (*int64, error) {
	var proposalID sql.NullInt64
	if err := q.QueryRow(`
		SELECT proposal_id
		FROM curriculum_projection_state
		WHERE singleton = TRUE
	` + lock).Scan(&proposalID); err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	if !proposalID.Valid {
		return nil, nil
	}
	return &proposalID.Int64, nil
}

func LockCurriculumProposal(q curriculumExecutor, proposalID int64) (bool, error) {
	var id int64
	err := q.QueryRow(`
		SELECT id
		FROM curriculum_proposals
		WHERE id = $1
		FOR UPDATE
	`, proposalID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock curriculum proposal: %w", err)
	}
	return true, nil
}

func CreateDraftCurriculumProposal(q curriculumExecutor, proposal *models.CurriculumProposal) error {
	err := q.QueryRow(`
		INSERT INTO curriculum_proposals (title, rationale, status, base_proposal_id)
		VALUES ($1, $2, 'draft', $3)
		RETURNING id, created_at
	`, proposal.Title, proposal.Rationale, proposal.BaseProposalID).
		Scan(&proposal.ID, &proposal.CreatedAt)
	if err != nil {
		return fmt.Errorf("create draft curriculum proposal: %w", err)
	}
	if _, err := q.Exec(`
		INSERT INTO curriculum_proposal_authors (proposal_id, user_id)
		VALUES ($1, $2)
	`, proposal.ID, proposal.AuthorIDs[0]); err != nil {
		return fmt.Errorf("add draft curriculum proposal author: %w", err)
	}
	proposal.Status = "draft"
	return nil
}

func GetCurriculumProposal(q curriculumExecutor, proposalID int64) (*models.CurriculumProposal, error) {
	var proposal models.CurriculumProposal
	var baseProposalID sql.NullInt64
	var acceptedAt, submittedAt, decidedAt sql.NullTime
	err := q.QueryRow(`
		SELECT proposal.id, authors.ids, authors.names,
		       proposal.title, proposal.rationale, proposal.status, proposal.base_proposal_id,
		       proposal.created_at, proposal.accepted_at, proposal.submitted_at,
		       proposal.decided_at
		FROM curriculum_proposals proposal
		JOIN LATERAL (
			SELECT array_agg(user_id ORDER BY users.full_name, user_id) AS ids,
			       string_agg(users.full_name, ', ' ORDER BY users.full_name, user_id) AS names
			FROM curriculum_proposal_authors
			JOIN users ON users.id = user_id
			WHERE proposal_id = proposal.id
		) authors ON TRUE
		WHERE proposal.id = $1
	`, proposalID).Scan(
		&proposal.ID, pq.Array(&proposal.AuthorIDs), &proposal.AuthorName, &proposal.Title,
		&proposal.Rationale, &proposal.Status, &baseProposalID,
		&proposal.CreatedAt, &acceptedAt, &submittedAt, &decidedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get curriculum proposal: %w", err)
	}
	if baseProposalID.Valid {
		proposal.BaseProposalID = &baseProposalID.Int64
	}
	if acceptedAt.Valid {
		proposal.AcceptedAt = &acceptedAt.Time
	}
	if submittedAt.Valid {
		proposal.SubmittedAt = &submittedAt.Time
	}
	if decidedAt.Valid {
		proposal.DecidedAt = &decidedAt.Time
	}
	changes, err := listCurriculumProposalChanges(q, proposal.ID)
	if err != nil {
		return nil, err
	}
	proposal.Changes = changes
	return &proposal, nil
}

func GetCurriculumUnitAtProposal(
	ctx context.Context,
	q contextualCurriculumExecutor,
	proposalID, unitID int64,
) (*models.Unit, error) {
	rows, err := q.QueryContext(ctx, `
		WITH RECURSIVE lineage AS (
			SELECT id, base_proposal_id, 0 AS depth
			FROM curriculum_proposals
			WHERE id = $1 AND status = 'accepted'
			UNION ALL
			SELECT proposal.id, proposal.base_proposal_id, lineage.depth + 1
			FROM curriculum_proposals proposal
			JOIN lineage ON proposal.id = lineage.base_proposal_id
			WHERE proposal.status = 'accepted'
		)
		SELECT change.kind, COALESCE(change.unit_name, ''), COALESCE(change.unit_content, '')
		FROM lineage
		JOIN curriculum_proposal_change_details change ON change.proposal_id = lineage.id
		WHERE change.unit_id = $2
		  AND change.kind IN ('create_unit', 'rename_unit', 'update_content', 'delete_unit')
		ORDER BY lineage.depth DESC,
		         CASE change.kind
		           WHEN 'create_unit' THEN 1
		           WHEN 'rename_unit' THEN 2
		           WHEN 'update_content' THEN 2
		           WHEN 'delete_unit' THEN 6
		         END,
		         change.id
	`, proposalID, unitID)
	if err != nil {
		return nil, fmt.Errorf("load curriculum unit history: %w", err)
	}
	defer rows.Close()
	var unit *models.Unit
	for rows.Next() {
		var kind, name, content string
		if err := rows.Scan(&kind, &name, &content); err != nil {
			return nil, fmt.Errorf("scan curriculum unit history: %w", err)
		}
		switch kind {
		case "create_unit":
			unit = &models.Unit{ID: unitID, Name: name, Content: content}
		case "rename_unit":
			if unit != nil {
				unit.Name = name
			}
		case "update_content":
			if unit != nil {
				unit.Content = content
			}
		case "delete_unit":
			unit = nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate curriculum unit history: %w", err)
	}
	return unit, nil
}

func GetCurriculumGraphAtProposal(
	ctx context.Context,
	q contextualCurriculumExecutor,
	proposalID int64,
) (*models.CurriculumGraph, error) {
	rows, err := q.QueryContext(ctx, `
		WITH RECURSIVE lineage AS (
			SELECT id, base_proposal_id, 0 AS depth
			FROM curriculum_proposals
			WHERE id = $1 AND status = 'accepted'
			UNION ALL
			SELECT proposal.id, proposal.base_proposal_id, lineage.depth + 1
			FROM curriculum_proposals proposal
			JOIN lineage ON proposal.id = lineage.base_proposal_id
			WHERE proposal.status = 'accepted'
		)
		SELECT change.kind, change.unit_id,
		       COALESCE(change.unit_name, ''), COALESCE(change.prerequisite_id, 0)
		FROM lineage
		JOIN curriculum_proposal_change_details change ON change.proposal_id = lineage.id
		WHERE change.kind IN (
			'create_unit', 'rename_unit', 'remove_dependency',
			'add_dependency', 'delete_unit'
		)
		ORDER BY lineage.depth DESC,
		         CASE change.kind
		           WHEN 'create_unit' THEN 1
		           WHEN 'rename_unit' THEN 2
		           WHEN 'remove_dependency' THEN 3
		           WHEN 'add_dependency' THEN 4
		           WHEN 'delete_unit' THEN 6
		         END,
		         change.unit_id, change.prerequisite_id NULLS LAST, change.id
	`, proposalID)
	if err != nil {
		return nil, fmt.Errorf("load historical curriculum graph: %w", err)
	}
	defer rows.Close()
	units := make(map[int64]models.Unit)
	dependencies := make(map[[2]int64]bool)
	for rows.Next() {
		var kind, name string
		var unitID, prerequisiteID int64
		if err := rows.Scan(&kind, &unitID, &name, &prerequisiteID); err != nil {
			return nil, fmt.Errorf("scan historical curriculum graph: %w", err)
		}
		switch kind {
		case "create_unit":
			units[unitID] = models.Unit{ID: unitID, Name: name}
		case "rename_unit":
			if unit, exists := units[unitID]; exists {
				unit.Name = name
				units[unitID] = unit
			}
		case "remove_dependency":
			delete(dependencies, [2]int64{unitID, prerequisiteID})
		case "add_dependency":
			dependencies[[2]int64{unitID, prerequisiteID}] = true
		case "delete_unit":
			delete(units, unitID)
			for edge := range dependencies {
				if edge[0] == unitID || edge[1] == unitID {
					delete(dependencies, edge)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate historical curriculum graph: %w", err)
	}
	graph := &models.CurriculumGraph{Units: make([]models.Unit, 0, len(units))}
	for _, unit := range units {
		graph.Units = append(graph.Units, unit)
	}
	sort.Slice(graph.Units, func(i, j int) bool {
		left, right := strings.ToLower(graph.Units[i].Name), strings.ToLower(graph.Units[j].Name)
		if left == right {
			return graph.Units[i].ID < graph.Units[j].ID
		}
		return left < right
	})
	for edge := range dependencies {
		dependent, dependentExists := units[edge[0]]
		prerequisite, prerequisiteExists := units[edge[1]]
		if !dependentExists || !prerequisiteExists {
			continue
		}
		graph.Dependencies = append(graph.Dependencies, models.UnitDependency{
			UnitID: edge[0], UnitName: dependent.Name,
			PrerequisiteID: edge[1], PrerequisiteName: prerequisite.Name,
		})
	}
	sort.Slice(graph.Dependencies, func(i, j int) bool {
		left, right := graph.Dependencies[i], graph.Dependencies[j]
		if left.UnitID == right.UnitID {
			return left.PrerequisiteID < right.PrerequisiteID
		}
		return left.UnitID < right.UnitID
	})
	return graph, nil
}

func UpdateDraftCurriculumProposal(q curriculumExecutor, proposalID, authorID int64, title, rationale string) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals SET title = $3, rationale = $4
		WHERE id = $1 AND status = 'draft'
		  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = id AND user_id = $2)
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
		WHERE id = $1 AND status = 'draft'
		  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = id AND user_id = $2)
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
		INSERT INTO curriculum_proposal_changes (proposal_id, kind)
		SELECT proposal.id, $3
		FROM curriculum_proposals proposal
		WHERE proposal.id = $1 AND proposal.status = 'draft'
		  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = proposal.id AND user_id = $2)
		RETURNING id
	`, proposalID, authorID, change.Kind).Scan(&change.ID)
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
		  AND proposal.id = change.proposal_id
		  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = proposal.id AND user_id = $3)
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
			WHERE id = $1 AND status = 'draft'
			  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = id AND user_id = $2)
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

func UpdateDraftCurriculumUnitCreation(
	q curriculumExecutor, proposalID, authorID, changeID int64, name, content string,
) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_unit_creations creation
		SET name = $4, content = $5
		FROM curriculum_proposal_changes change, curriculum_proposals proposal
		WHERE creation.change_id = $3
		  AND change.id = creation.change_id
		  AND change.kind = 'create_unit'
		  AND change.proposal_id = $1
		  AND proposal.id = change.proposal_id
		  AND proposal.status = 'draft'
		  AND EXISTS (
		      SELECT 1 FROM curriculum_proposal_authors
		      WHERE proposal_id = proposal.id AND user_id = $2
		  )
	`, proposalID, authorID, changeID, name, content)
	if err != nil {
		return false, fmt.Errorf("update draft curriculum unit creation: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func SubmitDraftCurriculumProposal(q curriculumExecutor, proposalID, authorID int64) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals
		SET status = 'submitted', submitted_at = clock_timestamp()
		WHERE id = $1 AND status = 'draft'
		  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = id AND user_id = $2)
	`, proposalID, authorID)
	if err != nil {
		return false, fmt.Errorf("submit draft curriculum proposal: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func AcceptSubmittedCurriculumProposal(q curriculumExecutor, proposalID int64) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals
		SET status = 'accepted', accepted_at = statement_timestamp(),
		    decided_at = statement_timestamp()
		WHERE id = $1 AND status = 'submitted'
	`, proposalID)
	if err != nil {
		return false, fmt.Errorf("accept submitted curriculum proposal: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func RejectSubmittedCurriculumProposal(q curriculumExecutor, proposalID int64) (bool, error) {
	result, err := q.Exec(`
		UPDATE curriculum_proposals
		SET status = 'rejected', decided_at = clock_timestamp()
		WHERE id = $1 AND status = 'submitted'
	`, proposalID)
	if err != nil {
		return false, fmt.Errorf("reject submitted curriculum proposal: %w", err)
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
		return nil
	case "update_content":
		result, err = q.Exec(`
			UPDATE curriculum_unit_content_updates content_update
			SET content = $2
			FROM curriculum_proposal_changes change, curriculum_proposals proposal
			WHERE content_update.change_id = $1
			  AND change.id = content_update.change_id
			  AND proposal.id = change.proposal_id
			  AND proposal.status = 'draft'
		`, change.ID, change.UnitContent)
	case "delete_unit":
		return nil
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

func ListAcceptedCurriculumProposals(
	database *sql.DB, limit, offset int,
) ([]models.CurriculumProposal, int, error) {
	var total int
	if err := database.QueryRow(`
		SELECT count(*) FROM curriculum_proposals WHERE status = 'accepted'
	`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count accepted curriculum proposals: %w", err)
	}
	rows, err := database.Query(`
		SELECT proposal.id, authors.ids, authors.names,
		       proposal.title, proposal.rationale, proposal.status,
		       proposal.base_proposal_id,
		       proposal.created_at, proposal.accepted_at,
		       count(change.id)
		FROM curriculum_proposals proposal
		JOIN LATERAL (
			SELECT array_agg(user_id ORDER BY users.full_name, user_id) AS ids,
			       string_agg(users.full_name, ', ' ORDER BY users.full_name, user_id) AS names
			FROM curriculum_proposal_authors
			JOIN users ON users.id = user_id
			WHERE proposal_id = proposal.id
		) authors ON TRUE
		LEFT JOIN curriculum_proposal_changes change ON change.proposal_id = proposal.id
		WHERE proposal.status = 'accepted'
		GROUP BY proposal.id, authors.ids, authors.names
		ORDER BY proposal.accepted_at DESC, proposal.id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list accepted curriculum proposals: %w", err)
	}
	defer rows.Close()
	var proposals []models.CurriculumProposal
	for rows.Next() {
		var proposal models.CurriculumProposal
		var baseProposalID sql.NullInt64
		var acceptedAt sql.NullTime
		if err := rows.Scan(
			&proposal.ID, pq.Array(&proposal.AuthorIDs), &proposal.AuthorName,
			&proposal.Title, &proposal.Rationale, &proposal.Status,
			&baseProposalID, &proposal.CreatedAt, &acceptedAt, &proposal.ChangeCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan accepted curriculum proposal: %w", err)
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
		return nil, 0, fmt.Errorf("iterate accepted curriculum proposals: %w", err)
	}
	return proposals, total, nil
}

func ListDraftCurriculumProposalsByAuthor(database *sql.DB, authorID int64) ([]models.CurriculumProposal, error) {
	rows, err := database.Query(`
		SELECT proposal.id, authors.ids, authors.names,
		       proposal.title, proposal.rationale, proposal.status,
		       proposal.base_proposal_id, proposal.created_at,
		       COUNT(change.id),
		       COALESCE(
		           ARRAY_AGG(change.kind ORDER BY change.id)
		               FILTER (WHERE change.id IS NOT NULL),
		           '{}'::TEXT[]
		       )
		FROM curriculum_proposals proposal
		JOIN LATERAL (
			SELECT array_agg(user_id ORDER BY users.full_name, user_id) AS ids,
			       string_agg(users.full_name, ', ' ORDER BY users.full_name, user_id) AS names
			FROM curriculum_proposal_authors
			JOIN users ON users.id = user_id
			WHERE proposal_id = proposal.id
		) authors ON TRUE
		LEFT JOIN curriculum_proposal_changes change ON change.proposal_id = proposal.id
		WHERE proposal.status = 'draft'
		  AND EXISTS (SELECT 1 FROM curriculum_proposal_authors WHERE proposal_id = proposal.id AND user_id = $1)
		GROUP BY proposal.id, authors.ids, authors.names
		ORDER BY proposal.created_at DESC
	`, authorID)
	if err != nil {
		return nil, fmt.Errorf("list draft curriculum proposals by author: %w", err)
	}
	defer rows.Close()
	var proposals []models.CurriculumProposal
	for rows.Next() {
		var proposal models.CurriculumProposal
		var baseProposalID sql.NullInt64
		var changeKinds []string
		if err := rows.Scan(
			&proposal.ID, pq.Array(&proposal.AuthorIDs), &proposal.AuthorName, &proposal.Title,
			&proposal.Rationale, &proposal.Status, &baseProposalID,
			&proposal.CreatedAt, &proposal.ChangeCount, pq.Array(&changeKinds),
		); err != nil {
			return nil, fmt.Errorf("scan draft curriculum proposal: %w", err)
		}
		if baseProposalID.Valid {
			proposal.BaseProposalID = &baseProposalID.Int64
		}
		proposal.ChangeKindCounts = make(map[string]int, len(changeKinds))
		for _, kind := range changeKinds {
			proposal.ChangeKindCounts[kind]++
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft curriculum proposals: %w", err)
	}
	return proposals, nil
}

func ListCurriculumProposalsForUser(
	database *sql.DB, userID int64, isAdmin bool, status string, limit, offset int,
) ([]models.CurriculumProposal, int, error) {
	return listCurriculumProposals(database, userID, isAdmin, status, nil, limit, offset)
}

func ListSubmittedCurriculumProposals(database *sql.DB, limit, offset int) ([]models.CurriculumProposal, int, error) {
	return listCurriculumProposals(database, 0, false, "submitted", nil, limit, offset)
}

func ListCurriculumProposalsByAuthor(database *sql.DB, authorID int64, limit, offset int) ([]models.CurriculumProposal, int, error) {
	return listCurriculumProposals(database, 0, true, "", &authorID, limit, offset)
}

func ListRejectedCurriculumProposalsByAuthor(database *sql.DB, authorID int64, limit, offset int) ([]models.CurriculumProposal, int, error) {
	return listCurriculumProposals(database, authorID, false, "rejected", &authorID, limit, offset)
}

func listCurriculumProposals(
	database *sql.DB, userID int64, isAdmin bool, status string, authorID *int64, limit, offset int,
) ([]models.CurriculumProposal, int, error) {
	statusFilter := any(nil)
	if status != "" {
		statusFilter = status
	}
	authorFilter := any(nil)
	if authorID != nil {
		authorFilter = *authorID
	}
	var total int
	if err := database.QueryRow(`
		SELECT count(*)
		FROM curriculum_proposals proposal
		WHERE ($2::TEXT IS NULL OR proposal.status = $2)
		  AND (
			proposal.status IN ('submitted', 'accepted') OR $3
			OR EXISTS (
				SELECT 1 FROM curriculum_proposal_authors
				WHERE proposal_id = proposal.id AND user_id = $1
			)
		  )
		  AND ($4::BIGINT IS NULL OR EXISTS (
			SELECT 1 FROM curriculum_proposal_authors
			WHERE proposal_id = proposal.id AND user_id = $4
		  ))
	`, userID, statusFilter, isAdmin, authorFilter).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count visible curriculum proposals: %w", err)
	}
	rows, err := database.Query(`
		SELECT proposal.id, authors.ids, authors.names,
		       proposal.title, proposal.rationale, proposal.status,
		       proposal.base_proposal_id, proposal.created_at, proposal.accepted_at,
		       proposal.submitted_at, proposal.decided_at,
		       count(change.id)
		FROM curriculum_proposals proposal
		JOIN LATERAL (
			SELECT array_agg(user_id ORDER BY users.full_name, user_id) AS ids,
			       string_agg(users.full_name, ', ' ORDER BY users.full_name, user_id) AS names
			FROM curriculum_proposal_authors
			JOIN users ON users.id = user_id
			WHERE proposal_id = proposal.id
		) authors ON TRUE
		LEFT JOIN curriculum_proposal_changes change ON change.proposal_id = proposal.id
		WHERE ($2::TEXT IS NULL OR proposal.status = $2)
		  AND (
			proposal.status IN ('submitted', 'accepted') OR $3
			OR EXISTS (
				SELECT 1 FROM curriculum_proposal_authors
				WHERE proposal_id = proposal.id AND user_id = $1
			)
		  )
		  AND ($4::BIGINT IS NULL OR EXISTS (
			SELECT 1 FROM curriculum_proposal_authors
			WHERE proposal_id = proposal.id AND user_id = $4
		  ))
		GROUP BY proposal.id, authors.ids, authors.names
		ORDER BY proposal.created_at DESC, proposal.id DESC
		LIMIT $5 OFFSET $6
	`, userID, statusFilter, isAdmin, authorFilter, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list visible curriculum proposals: %w", err)
	}
	defer rows.Close()
	var proposals []models.CurriculumProposal
	for rows.Next() {
		var proposal models.CurriculumProposal
		var baseProposalID sql.NullInt64
		var acceptedAt, submittedAt, decidedAt sql.NullTime
		if err := rows.Scan(
			&proposal.ID, pq.Array(&proposal.AuthorIDs), &proposal.AuthorName,
			&proposal.Title, &proposal.Rationale, &proposal.Status,
			&baseProposalID, &proposal.CreatedAt, &acceptedAt, &submittedAt, &decidedAt,
			&proposal.ChangeCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan visible curriculum proposal: %w", err)
		}
		if baseProposalID.Valid {
			proposal.BaseProposalID = &baseProposalID.Int64
		}
		if acceptedAt.Valid {
			proposal.AcceptedAt = &acceptedAt.Time
		}
		if submittedAt.Valid {
			proposal.SubmittedAt = &submittedAt.Time
		}
		if decidedAt.Valid {
			proposal.DecidedAt = &decidedAt.Time
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate visible curriculum proposals: %w", err)
	}
	return proposals, total, nil
}
