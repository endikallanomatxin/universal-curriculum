package db

import "fmt"

func CompletedUnitIDs(q curriculumExecutor, userID int64) (map[int64]bool, error) {
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

	ids := make(map[int64]bool)
	for rows.Next() {
		var unitID int64
		if err := rows.Scan(&unitID); err != nil {
			return nil, fmt.Errorf("scan completed unit id: %w", err)
		}
		ids[unitID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completed unit ids: %w", err)
	}
	return ids, nil
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
