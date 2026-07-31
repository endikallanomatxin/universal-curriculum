package db

import (
	"fmt"

	"universal-curriculum/internal/models"
)

func CompletedUnitIDs(q curriculumExecutor, userID int64) (map[int64]bool, error) {
	statuses, err := UnitCompletionStatuses(q, userID)
	if err != nil {
		return nil, err
	}
	ids := make(map[int64]bool, len(statuses))
	for unitID, status := range statuses {
		if status.Completed() {
			ids[unitID] = true
		}
	}
	return ids, nil
}

func UnitCompletionStatuses(q curriculumExecutor, userID int64) (map[int64]models.UnitCompletionStatus, error) {
	rows, err := q.Query(`
		SELECT unit_id
		FROM completed_units
		WHERE user_id = $1
		ORDER BY unit_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list completed unit ids: %w", err)
	}
	defer rows.Close()

	statuses := make(map[int64]models.UnitCompletionStatus)
	for rows.Next() {
		var unitID int64
		if err := rows.Scan(&unitID); err != nil {
			return nil, fmt.Errorf("scan completed unit id: %w", err)
		}
		statuses[unitID] = models.UnitCompletionStatus{Direct: true}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completed unit ids: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close completed unit ids: %w", err)
	}
	transfers, err := acceptedKnowledgeTransfers(q)
	if err != nil {
		return nil, err
	}
	applyKnowledgeTransfers(statuses, transfers)
	return statuses, nil
}

func applyKnowledgeTransfers(
	statuses map[int64]models.UnitCompletionStatus,
	transfers []models.KnowledgeTransfer,
) {
	for changed := true; changed; {
		changed = false
		for _, transfer := range transfers {
			allSourcesCompleted := true
			for _, source := range transfer.Sources {
				if !statuses[source.ID].Completed() {
					allSourcesCompleted = false
					break
				}
			}
			if !allSourcesCompleted {
				continue
			}
			for _, target := range transfer.Targets {
				status := statuses[target.ID]
				if status.Completed() {
					continue
				}
				status.Transferred = true
				statuses[target.ID] = status
				changed = true
			}
		}
	}
}

func SetUnitCompleted(q curriculumExecutor, userID, unitID int64, completed bool) error {
	if completed {
		if _, err := q.Exec(`
			INSERT INTO completed_units (user_id, unit_id, curriculum_proposal_id)
			SELECT $1, $2, proposal_id
			FROM curriculum_projection_state
			WHERE singleton = TRUE AND proposal_id IS NOT NULL
			ON CONFLICT (user_id, unit_id) DO NOTHING
		`, userID, unitID); err != nil {
			return fmt.Errorf("complete unit: %w", err)
		}
		return nil
	}
	if _, err := q.Exec(`
		DELETE FROM completed_units
		WHERE user_id = $1 AND unit_id = $2
	`, userID, unitID); err != nil {
		return fmt.Errorf("uncomplete unit: %w", err)
	}
	return nil
}

func acceptedKnowledgeTransfers(q curriculumExecutor) ([]models.KnowledgeTransfer, error) {
	rows, err := q.Query(`
		WITH RECURSIVE proposal_lineage AS (
			SELECT proposal.id, proposal.base_proposal_id
			FROM curriculum_projection_state state
			JOIN curriculum_proposals proposal ON proposal.id = state.proposal_id
			WHERE state.singleton = TRUE AND proposal.status = 'accepted'

			UNION ALL

			SELECT proposal.id, proposal.base_proposal_id
			FROM curriculum_proposals proposal
			JOIN proposal_lineage lineage ON proposal.id = lineage.base_proposal_id
			WHERE proposal.status = 'accepted'
		),
		members AS (
			SELECT source.transfer_change_id, 'source'::TEXT AS role, source.unit_id
			FROM curriculum_knowledge_transfer_sources source
			UNION ALL
			SELECT target.transfer_change_id, 'target'::TEXT AS role, target.unit_id
			FROM curriculum_knowledge_transfer_targets target
		)
		SELECT transfer.change_id, member.role, member.unit_id
		FROM proposal_lineage lineage
		JOIN curriculum_proposal_changes change ON change.proposal_id = lineage.id
		JOIN curriculum_knowledge_transfers transfer ON transfer.change_id = change.id
		JOIN members member ON member.transfer_change_id = transfer.change_id
		ORDER BY transfer.change_id, member.role, member.unit_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list accepted curriculum knowledge transfers: %w", err)
	}
	defer rows.Close()
	var transfers []models.KnowledgeTransfer
	indexes := make(map[int64]int)
	for rows.Next() {
		var changeID, unitID int64
		var role string
		if err := rows.Scan(&changeID, &role, &unitID); err != nil {
			return nil, fmt.Errorf("scan accepted curriculum knowledge transfer: %w", err)
		}
		index, exists := indexes[changeID]
		if !exists {
			index = len(transfers)
			indexes[changeID] = index
			transfers = append(transfers, models.KnowledgeTransfer{})
		}
		if role == "source" {
			transfers[index].Sources = append(transfers[index].Sources, models.Unit{ID: unitID})
		} else {
			transfers[index].Targets = append(transfers[index].Targets, models.Unit{ID: unitID})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accepted curriculum knowledge transfers: %w", err)
	}
	return transfers, nil
}
