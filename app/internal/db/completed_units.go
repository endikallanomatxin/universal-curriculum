package db

import (
	"database/sql"
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
		)
		SELECT completion.unit_id,
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
		FROM unit_completions completion
		WHERE completion.user_id = $1
		ORDER BY completion.unit_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list directly completed unit ids: %w", err)
	}
	statuses := make(map[int64]models.UnitCompletionStatus)
	for rows.Next() {
		var unitID int64
		var changedSinceCompletion bool
		if err := rows.Scan(&unitID, &changedSinceCompletion); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan directly completed unit id: %w", err)
		}
		statuses[unitID] = models.UnitCompletionStatus{
			Direct: !changedSinceCompletion, Recognized: changedSinceCompletion,
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate directly completed unit ids: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close directly completed unit ids: %w", err)
	}

	rows, err = q.Query(`
		SELECT DISTINCT unit_id
		FROM unit_completion_recognitions
		WHERE user_id = $1
		ORDER BY unit_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list recognized unit ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var unitID int64
		if err := rows.Scan(&unitID); err != nil {
			return nil, fmt.Errorf("scan recognized unit id: %w", err)
		}
		status := statuses[unitID]
		if !status.Direct {
			status.Recognized = true
		}
		statuses[unitID] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recognized unit ids: %w", err)
	}
	return statuses, nil
}

func SetUnitCompleted(database *sql.DB, userID, unitID int64, completed bool) error {
	if !completed {
		tx, err := database.Begin()
		if err != nil {
			return fmt.Errorf("begin returning curriculum unit to pending: %w", err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`
			DELETE FROM unit_completions
			WHERE user_id = $1 AND unit_id = $2
		`, userID, unitID); err != nil {
			return fmt.Errorf("return curriculum unit to pending: %w", err)
		}
		if _, err := tx.Exec(`
			DELETE FROM unit_completion_recognitions
			WHERE user_id = $1 AND unit_id = $2
		`, userID, unitID); err != nil {
			return fmt.Errorf("remove recognized curriculum completion: %w", err)
		}
		if err := PruneUserCurriculumRecognitions(tx, userID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit returning curriculum unit to pending: %w", err)
		}
		return nil
	}
	if _, err := database.Exec(`
		INSERT INTO unit_completions (user_id, unit_id, curriculum_proposal_id)
		SELECT $1, $2, state.proposal_id
		FROM curriculum_projection_state state
		JOIN units unit ON unit.id = $2
		WHERE state.singleton = TRUE AND state.proposal_id IS NOT NULL
		ON CONFLICT (user_id, unit_id) DO UPDATE
		SET curriculum_proposal_id = EXCLUDED.curriculum_proposal_id
		WHERE unit_completions.curriculum_proposal_id IS DISTINCT FROM EXCLUDED.curriculum_proposal_id
	`, userID, unitID); err != nil {
		return fmt.Errorf("set curriculum unit completed: %w", err)
	}
	return nil
}

// PruneUserCurriculumRecognitions removes materialized results whose source
// evidence is no longer present. It never adds evidence, so a learner can
// softly return a recognized unit to pending without it being immediately
// regenerated. Source evidence must still predate the recognition that used it.
func PruneUserCurriculumRecognitions(q curriculumExecutor, userID int64) error {
	for {
		result, err := q.Exec(`
			WITH RECURSIVE proposal_ancestry AS (
				SELECT proposal.id AS descendant_id, proposal.base_proposal_id AS ancestor_id
				FROM curriculum_proposals proposal
				WHERE proposal.status = 'accepted' AND proposal.base_proposal_id IS NOT NULL

				UNION ALL

				SELECT ancestry.descendant_id, proposal.base_proposal_id
				FROM proposal_ancestry ancestry
				JOIN curriculum_proposals proposal ON proposal.id = ancestry.ancestor_id
				WHERE proposal.base_proposal_id IS NOT NULL
			), unsupported AS (
				SELECT recognized.user_id, recognized.unit_id, recognized.recognition_change_id
				FROM unit_completion_recognitions recognized
				JOIN curriculum_proposal_changes recognition_change
				  ON recognition_change.id = recognized.recognition_change_id
				WHERE recognized.user_id = $1
				  AND EXISTS (
					SELECT 1
					FROM curriculum_recognition_sources source
					WHERE source.recognition_change_id = recognized.recognition_change_id
					  AND NOT EXISTS (
						SELECT 1
						FROM unit_completions direct
						JOIN proposal_ancestry ancestry
						  ON ancestry.descendant_id = recognition_change.proposal_id
						 AND ancestry.ancestor_id = direct.curriculum_proposal_id
						WHERE direct.user_id = recognized.user_id
						  AND direct.unit_id = source.unit_id
					  )
					  AND NOT EXISTS (
						SELECT 1
						FROM unit_completion_recognitions source_recognition
						JOIN curriculum_proposal_changes source_change
						  ON source_change.id = source_recognition.recognition_change_id
						JOIN proposal_ancestry ancestry
						  ON ancestry.descendant_id = recognition_change.proposal_id
						 AND ancestry.ancestor_id = source_change.proposal_id
						WHERE source_recognition.user_id = recognized.user_id
						  AND source_recognition.unit_id = source.unit_id
					  )
				  )
			)
			DELETE FROM unit_completion_recognitions recognized
			USING unsupported
			WHERE recognized.user_id = unsupported.user_id
			  AND recognized.unit_id = unsupported.unit_id
			  AND recognized.recognition_change_id = unsupported.recognition_change_id
		`, userID)
		if err != nil {
			return fmt.Errorf("prune unsupported curriculum recognitions: %w", err)
		}
		deleted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read pruned curriculum recognition count: %w", err)
		}
		if deleted == 0 {
			return nil
		}
	}
}

// MaterializeCurriculumRecognitions records the derived progress granted by
// every recognition in one newly accepted proposal. All rules read the same
// pre-proposal evidence snapshot, so recognitions in the same proposal cannot
// feed one another; later proposals can consume the persisted results.
func MaterializeCurriculumRecognitions(q curriculumExecutor, proposalID int64) error {
	return materializeCurriculumRecognitions(q, proposalID, nil)
}

func materializeCurriculumRecognitions(q curriculumExecutor, proposalID int64, userID *int64) error {
	if _, err := q.Exec(`
		WITH RECURSIVE prior_lineage AS (
			SELECT base.id, base.base_proposal_id
			FROM curriculum_proposals proposal
			JOIN curriculum_proposals base ON base.id = proposal.base_proposal_id
			WHERE proposal.id = $1 AND proposal.status = 'accepted'

			UNION ALL

			SELECT base.id, base.base_proposal_id
			FROM curriculum_proposals base
			JOIN prior_lineage later ON base.id = later.base_proposal_id
		), evidence AS MATERIALIZED (
			SELECT completion.user_id, completion.unit_id
			FROM unit_completions completion
			JOIN prior_lineage ON prior_lineage.id = completion.curriculum_proposal_id
			UNION
			SELECT recognized.user_id, recognized.unit_id
			FROM unit_completion_recognitions recognized
			JOIN curriculum_proposal_changes recognition_change
			  ON recognition_change.id = recognized.recognition_change_id
			JOIN prior_lineage ON prior_lineage.id = recognition_change.proposal_id
		), eligible AS (
			SELECT recognition.change_id, evidence.user_id
			FROM curriculum_proposal_changes change
			JOIN curriculum_recognitions recognition ON recognition.change_id = change.id
			JOIN curriculum_recognition_sources source
			  ON source.recognition_change_id = recognition.change_id
			JOIN evidence ON evidence.unit_id = source.unit_id
			WHERE change.proposal_id = $1
			  AND ($2::BIGINT IS NULL OR evidence.user_id = $2)
			GROUP BY recognition.change_id, evidence.user_id
			HAVING count(DISTINCT source.unit_id) = (
				SELECT count(*)
				FROM curriculum_recognition_sources required
				WHERE required.recognition_change_id = recognition.change_id
			)
		)
		INSERT INTO unit_completion_recognitions (
			user_id, unit_id, recognition_change_id
		)
		SELECT eligible.user_id, target.unit_id, eligible.change_id
		FROM eligible
		JOIN curriculum_recognition_targets target
		  ON target.recognition_change_id = eligible.change_id
		ON CONFLICT DO NOTHING
	`, proposalID, userID); err != nil {
		return fmt.Errorf("materialize curriculum recognitions: %w", err)
	}
	return nil
}

// RebuildUserCurriculumRecognitions restores one learner's derived projection
// from their remaining direct completions. Proposal groups are replayed oldest
// first so only evidence from strictly earlier curriculum states can feed each
// recognition.
func RebuildUserCurriculumRecognitions(q curriculumExecutor, userID int64) error {
	if _, err := q.Exec(`
		DELETE FROM unit_completion_recognitions WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("clear recognized curriculum progress: %w", err)
	}
	rows, err := q.Query(`
		WITH RECURSIVE proposal_lineage AS (
			SELECT proposal.id, proposal.base_proposal_id, 0 AS depth
			FROM curriculum_projection_state state
			JOIN curriculum_proposals proposal ON proposal.id = state.proposal_id
			WHERE state.singleton = TRUE AND proposal.status = 'accepted'

			UNION ALL

			SELECT proposal.id, proposal.base_proposal_id, lineage.depth + 1
			FROM curriculum_proposals proposal
			JOIN proposal_lineage lineage ON proposal.id = lineage.base_proposal_id
			WHERE proposal.status = 'accepted'
		)
		SELECT lineage.id
		FROM proposal_lineage lineage
		WHERE EXISTS (
			SELECT 1
			FROM curriculum_proposal_changes change
			JOIN curriculum_recognitions recognition ON recognition.change_id = change.id
			WHERE change.proposal_id = lineage.id
		)
		ORDER BY lineage.depth DESC
	`)
	if err != nil {
		return fmt.Errorf("list curriculum recognition history: %w", err)
	}
	var proposalIDs []int64
	for rows.Next() {
		var proposalID int64
		if err := rows.Scan(&proposalID); err != nil {
			rows.Close()
			return fmt.Errorf("scan curriculum recognition history: %w", err)
		}
		proposalIDs = append(proposalIDs, proposalID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate curriculum recognition history: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close curriculum recognition history: %w", err)
	}
	for _, proposalID := range proposalIDs {
		if err := materializeCurriculumRecognitions(q, proposalID, &userID); err != nil {
			return err
		}
	}
	return nil
}
