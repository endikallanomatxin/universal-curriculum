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
	recognitions, err := acceptedRecognitions(q)
	if err != nil {
		return nil, err
	}
	applyRecognitions(statuses, recognitions)
	return statuses, nil
}

func applyRecognitions(
	statuses map[int64]models.UnitCompletionStatus,
	recognitions []models.Recognition,
) {
	for changed := true; changed; {
		changed = false
		for _, recognition := range recognitions {
			allSourcesCompleted := true
			for _, source := range recognition.Sources {
				if !statuses[source.ID].Completed() {
					allSourcesCompleted = false
					break
				}
			}
			if !allSourcesCompleted {
				continue
			}
			for _, target := range recognition.Targets {
				status := statuses[target.ID]
				if status.Completed() {
					continue
				}
				status.Recognized = true
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

func acceptedRecognitions(q curriculumExecutor) ([]models.Recognition, error) {
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
			SELECT source.recognition_change_id, 'source'::TEXT AS role, source.unit_id
			FROM curriculum_recognition_sources source
			UNION ALL
			SELECT target.recognition_change_id, 'target'::TEXT AS role, target.unit_id
			FROM curriculum_recognition_targets target
		)
		SELECT recognition.change_id, member.role, member.unit_id
		FROM proposal_lineage lineage
		JOIN curriculum_proposal_changes change ON change.proposal_id = lineage.id
		JOIN curriculum_recognitions recognition ON recognition.change_id = change.id
		JOIN members member ON member.recognition_change_id = recognition.change_id
		ORDER BY recognition.change_id, member.role, member.unit_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list accepted curriculum recognitions: %w", err)
	}
	defer rows.Close()
	var recognitions []models.Recognition
	indexes := make(map[int64]int)
	for rows.Next() {
		var changeID, unitID int64
		var role string
		if err := rows.Scan(&changeID, &role, &unitID); err != nil {
			return nil, fmt.Errorf("scan accepted curriculum recognition: %w", err)
		}
		index, exists := indexes[changeID]
		if !exists {
			index = len(recognitions)
			indexes[changeID] = index
			recognitions = append(recognitions, models.Recognition{})
		}
		if role == "source" {
			recognitions[index].Sources = append(recognitions[index].Sources, models.Unit{ID: unitID})
		} else {
			recognitions[index].Targets = append(recognitions[index].Targets, models.Unit{ID: unitID})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accepted curriculum recognitions: %w", err)
	}
	return recognitions, nil
}
