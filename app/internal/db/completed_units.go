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
		WITH RECURSIVE proposal_lineage AS (
			SELECT proposal.id, proposal.base_proposal_id, 0 AS depth
			FROM curriculum_projection_state state
			JOIN curriculum_proposals proposal ON proposal.id = state.proposal_id
			WHERE state.singleton = TRUE

			UNION ALL

			SELECT proposal.id, proposal.base_proposal_id, lineage.depth + 1
			FROM curriculum_proposals proposal
			JOIN proposal_lineage lineage ON proposal.id = lineage.base_proposal_id
		), current_completion AS (
			SELECT DISTINCT ON (unit_id) unit_id, is_completed
			     , curriculum_proposal_id
			FROM unit_completion_events
			WHERE user_id = $1
			ORDER BY unit_id, id DESC
		)
		SELECT completion.unit_id, completion.is_completed,
		       EXISTS (
			SELECT 1
			FROM proposal_lineage completed_at
			JOIN proposal_lineage changed_after ON changed_after.depth < completed_at.depth
			JOIN curriculum_proposal_change_details change
			  ON change.proposal_id = changed_after.id
			WHERE completed_at.id = completion.curriculum_proposal_id
			  AND change.kind = 'update_content'
			  AND change.unit_id = completion.unit_id
		       ) AS changed_since_completion
		FROM current_completion completion
		ORDER BY completion.unit_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list completed unit ids: %w", err)
	}
	defer rows.Close()

	statuses := make(map[int64]models.UnitCompletionStatus)
	for rows.Next() {
		var unitID int64
		var completed, changedSinceCompletion bool
		if err := rows.Scan(&unitID, &completed, &changedSinceCompletion); err != nil {
			return nil, fmt.Errorf("scan completed unit id: %w", err)
		}
		if completed {
			statuses[unitID] = models.UnitCompletionStatus{
				Direct: !changedSinceCompletion, Recognized: changedSinceCompletion,
			}
		}
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
	if _, err := q.Exec(`
		INSERT INTO unit_completion_events (
			user_id, unit_id, curriculum_proposal_id, is_completed
		)
		SELECT $1, $2, state.proposal_id, $3
		FROM curriculum_projection_state state
		JOIN units unit ON unit.id = $2
		WHERE state.singleton = TRUE
		  AND state.proposal_id IS NOT NULL
		  AND ($3 OR EXISTS (
			SELECT 1 FROM unit_completion_events existing
			WHERE existing.user_id = $1 AND existing.unit_id = $2
		  ))
		  AND NOT EXISTS (
			SELECT 1
			FROM (
				SELECT event.is_completed, event.curriculum_proposal_id
				FROM unit_completion_events event
				WHERE event.user_id = $1 AND event.unit_id = $2
				ORDER BY event.id DESC
				LIMIT 1
			) latest
			WHERE latest.is_completed = $3
			  AND (NOT $3 OR latest.curriculum_proposal_id = state.proposal_id)
		  )
	`, userID, unitID, completed); err != nil {
		return fmt.Errorf("set unit completion: %w", err)
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
