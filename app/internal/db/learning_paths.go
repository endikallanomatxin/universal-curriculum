package db

import (
	"database/sql"
	"errors"
	"fmt"

	"universal-curriculum/internal/models"
)

func ListLearningPaths(database *sql.DB, userID int64) ([]models.LearningPath, error) {
	rows, err := database.Query(`
		SELECT path.id, path.user_id, path.name, path.description,
		       path.created_at, path.updated_at,
		       path_unit.unit_id, COALESCE(unit.name, creation.name),
		       unit.id IS NULL
		FROM learning_paths path
		LEFT JOIN learning_path_units path_unit ON path_unit.path_id = path.id
		LEFT JOIN units unit ON unit.id = path_unit.unit_id
		LEFT JOIN curriculum_unit_creations creation ON creation.change_id = path_unit.unit_id
		WHERE path.user_id = $1
		ORDER BY path.updated_at DESC, lower(path.name), path.id, path_unit.position
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list learning paths: %w", err)
	}
	defer rows.Close()
	var paths []models.LearningPath
	pathIndexes := make(map[int64]int)
	for rows.Next() {
		var path models.LearningPath
		var unitID sql.NullInt64
		var unitName sql.NullString
		var unitRetired sql.NullBool
		if err := rows.Scan(
			&path.ID, &path.UserID, &path.Name, &path.Description,
			&path.CreatedAt, &path.UpdatedAt,
			&unitID, &unitName, &unitRetired,
		); err != nil {
			return nil, fmt.Errorf("scan learning path: %w", err)
		}
		index, exists := pathIndexes[path.ID]
		if !exists {
			index = len(paths)
			pathIndexes[path.ID] = index
			paths = append(paths, path)
		}
		if unitID.Valid {
			paths[index].Units = append(paths[index].Units, models.Unit{
				ID:      unitID.Int64,
				Name:    unitName.String,
				Retired: unitRetired.Bool,
			})
			paths[index].UnitCount++
		}
	}
	return paths, rows.Err()
}

func GetLearningPath(q curriculumExecutor, userID, pathID int64) (*models.LearningPath, error) {
	var path models.LearningPath
	err := q.QueryRow(`
		SELECT id, user_id, name, description, created_at, updated_at
		FROM learning_paths
		WHERE id = $1 AND user_id = $2
	`, pathID, userID).Scan(
		&path.ID, &path.UserID, &path.Name, &path.Description, &path.CreatedAt, &path.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get learning path: %w", err)
	}
	rows, err := q.Query(`
		SELECT path_unit.unit_id,
		       COALESCE(unit.name, creation.name),
		       COALESCE(unit.content, creation.content),
		       COALESCE(unit.created_at, proposal.created_at),
		       COALESCE(unit.updated_at, proposal.created_at),
		       unit.id IS NULL
		FROM learning_path_units path_unit
		JOIN curriculum_unit_creations creation ON creation.change_id = path_unit.unit_id
		JOIN curriculum_proposal_changes change ON change.id = creation.change_id
		JOIN curriculum_proposals proposal ON proposal.id = change.proposal_id
		LEFT JOIN units unit ON unit.id = path_unit.unit_id
		WHERE path_unit.path_id = $1
		ORDER BY path_unit.position
	`, path.ID)
	if err != nil {
		return nil, fmt.Errorf("list learning path units: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var unit models.Unit
		if err := rows.Scan(
			&unit.ID, &unit.Name, &unit.Content, &unit.CreatedAt, &unit.UpdatedAt, &unit.Retired,
		); err != nil {
			return nil, fmt.Errorf("scan learning path unit: %w", err)
		}
		path.Units = append(path.Units, unit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate learning path units: %w", err)
	}
	return &path, nil
}

func InsertLearningPath(q curriculumExecutor, path *models.LearningPath) error {
	return q.QueryRow(`
		INSERT INTO learning_paths (user_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`, path.UserID, path.Name, path.Description).Scan(&path.ID, &path.CreatedAt, &path.UpdatedAt)
}

func UpdateLearningPath(q curriculumExecutor, path *models.LearningPath) (bool, error) {
	result, err := q.Exec(`
		UPDATE learning_paths
		SET name = $3, description = $4, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, path.ID, path.UserID, path.Name, path.Description)
	if err != nil {
		return false, fmt.Errorf("update learning path: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func ReplaceLearningPathUnits(q curriculumExecutor, pathID int64, unitIDs []int64) error {
	if _, err := q.Exec(`DELETE FROM learning_path_units WHERE path_id = $1`, pathID); err != nil {
		return fmt.Errorf("clear learning path units: %w", err)
	}
	for index, unitID := range unitIDs {
		if _, err := q.Exec(`
			INSERT INTO learning_path_units (path_id, unit_id, position)
			VALUES ($1, $2, $3)
		`, pathID, unitID, index+1); err != nil {
			return fmt.Errorf("add learning path unit: %w", err)
		}
	}
	return nil
}

func DeleteLearningPath(database *sql.DB, userID, pathID int64) (bool, error) {
	result, err := database.Exec(`DELETE FROM learning_paths WHERE id = $1 AND user_id = $2`, pathID, userID)
	if err != nil {
		return false, fmt.Errorf("delete learning path: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
