package db

import (
	"database/sql"
	"errors"
	"fmt"

	"universal-curriculum/internal/models"
)

func ListLearningPaths(database *sql.DB, userID int64) ([]models.LearningPath, error) {
	rows, err := database.Query(`
		SELECT path.id, path.user_id, path.name,
		       path.created_at, path.updated_at,
		       path_unit.unit_id, COALESCE(unit.name, creation.name),
		       unit.id IS NULL
		FROM learning_paths path
		LEFT JOIN learning_path_units path_unit ON path_unit.path_id = path.id
		LEFT JOIN units unit ON unit.id = path_unit.unit_id
		LEFT JOIN curriculum_unit_creations creation ON creation.change_id = path_unit.unit_id
		WHERE path.user_id = $1
		ORDER BY path.updated_at DESC, lower(path.name), path.id,
		         lower(COALESCE(unit.name, creation.name)), path_unit.unit_id
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
			&path.ID, &path.UserID, &path.Name,
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
		SELECT id, user_id, name, created_at, updated_at
		FROM learning_paths
		WHERE id = $1 AND user_id = $2
	`, pathID, userID).Scan(
		&path.ID, &path.UserID, &path.Name, &path.CreatedAt, &path.UpdatedAt,
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
		ORDER BY lower(COALESCE(unit.name, creation.name)), path_unit.unit_id
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
		INSERT INTO learning_paths (user_id, name)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`, path.UserID, path.Name).Scan(&path.ID, &path.CreatedAt, &path.UpdatedAt)
}

func UpdateLearningPath(q curriculumExecutor, path *models.LearningPath) (bool, error) {
	result, err := q.Exec(`
		UPDATE learning_paths
		SET name = $3, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, path.ID, path.UserID, path.Name)
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
	for _, unitID := range unitIDs {
		if _, err := q.Exec(`
			INSERT INTO learning_path_units (path_id, unit_id)
			VALUES ($1, $2)
		`, pathID, unitID); err != nil {
			return fmt.Errorf("add learning path unit: %w", err)
		}
	}
	return nil
}

// MigrateLearningPathTargets applies the recognitions in one accepted proposal
// to every learning path that targets one of their sources. The complete
// mapping is read before any path is changed, so recognitions accepted together
// cannot feed one another.
func MigrateLearningPathTargets(q curriculumExecutor, proposalID int64) error {
	rows, err := q.Query(`
		SELECT DISTINCT path_unit.path_id, source.unit_id, target.unit_id,
		       EXISTS (
			SELECT 1
			FROM curriculum_proposal_changes deletion_change
			JOIN curriculum_unit_deletions deletion ON deletion.change_id = deletion_change.id
			WHERE deletion_change.proposal_id = change.proposal_id
			  AND deletion.unit_id = source.unit_id
		       ) AS source_deleted
		FROM curriculum_proposal_changes change
		JOIN curriculum_recognition_sources source
		  ON source.recognition_change_id = change.id
		JOIN curriculum_recognition_targets target
		  ON target.recognition_change_id = change.id
		JOIN learning_path_units path_unit ON path_unit.unit_id = source.unit_id
		WHERE change.proposal_id = $1 AND change.kind = 'recognition'
		ORDER BY path_unit.path_id, source.unit_id, target.unit_id
	`, proposalID)
	if err != nil {
		return fmt.Errorf("list learning path target migrations: %w", err)
	}
	type targetMigration struct {
		deletedSources map[int64]bool
		targets        map[int64]bool
	}
	migrations := make(map[int64]*targetMigration)
	for rows.Next() {
		var pathID, sourceID, targetID int64
		var sourceDeleted bool
		if err := rows.Scan(&pathID, &sourceID, &targetID, &sourceDeleted); err != nil {
			rows.Close()
			return fmt.Errorf("scan learning path target migration: %w", err)
		}
		migration := migrations[pathID]
		if migration == nil {
			migration = &targetMigration{deletedSources: make(map[int64]bool), targets: make(map[int64]bool)}
			migrations[pathID] = migration
		}
		if sourceDeleted {
			migration.deletedSources[sourceID] = true
		}
		migration.targets[targetID] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate learning path target migrations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close learning path target migrations: %w", err)
	}
	for pathID, migration := range migrations {
		for sourceID := range migration.deletedSources {
			if _, err := q.Exec(`
				DELETE FROM learning_path_units WHERE path_id = $1 AND unit_id = $2
			`, pathID, sourceID); err != nil {
				return fmt.Errorf("remove recognized learning path target: %w", err)
			}
		}
		for targetID := range migration.targets {
			if _, err := q.Exec(`
				INSERT INTO learning_path_units (path_id, unit_id)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING
			`, pathID, targetID); err != nil {
				return fmt.Errorf("add recognized learning path target: %w", err)
			}
		}
		if _, err := q.Exec(`UPDATE learning_paths SET updated_at = NOW() WHERE id = $1`, pathID); err != nil {
			return fmt.Errorf("touch migrated learning path: %w", err)
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
