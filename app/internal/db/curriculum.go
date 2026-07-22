package db

import (
	"database/sql"
	"fmt"

	"universal-curriculum/internal/models"
)

type curriculumExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func GetCurriculumGraph(database *sql.DB) (*models.CurriculumGraph, error) {
	graph := &models.CurriculumGraph{}
	rows, err := database.Query(`
		SELECT id, name, description, created_at, updated_at
		FROM units
		ORDER BY lower(name), id
	`)
	if err != nil {
		return nil, fmt.Errorf("list curriculum units: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var unit models.Unit
		if err := rows.Scan(&unit.ID, &unit.Name, &unit.Description, &unit.CreatedAt, &unit.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan curriculum unit: %w", err)
		}
		graph.Units = append(graph.Units, unit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate curriculum units: %w", err)
	}

	dependencyRows, err := database.Query(`
		SELECT dependent.id, dependent.name, prerequisite.id, prerequisite.name
		FROM unit_dependencies dependency
		JOIN units dependent ON dependent.id = dependency.unit_id
		JOIN units prerequisite ON prerequisite.id = dependency.prerequisite_id
		ORDER BY lower(dependent.name), lower(prerequisite.name), dependent.id, prerequisite.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list unit dependencies: %w", err)
	}
	defer dependencyRows.Close()
	for dependencyRows.Next() {
		var dependency models.UnitDependency
		if err := dependencyRows.Scan(
			&dependency.UnitID, &dependency.UnitName,
			&dependency.PrerequisiteID, &dependency.PrerequisiteName,
		); err != nil {
			return nil, fmt.Errorf("scan unit dependency: %w", err)
		}
		graph.Dependencies = append(graph.Dependencies, dependency)
	}
	if err := dependencyRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unit dependencies: %w", err)
	}
	return graph, nil
}

func CreateUnit(database *sql.DB, name, description string) (*models.Unit, error) {
	var unit models.Unit
	err := database.QueryRow(`
		INSERT INTO units (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at, updated_at
	`, name, description).Scan(&unit.ID, &unit.Name, &unit.Description, &unit.CreatedAt, &unit.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create curriculum unit: %w", err)
	}
	return &unit, nil
}

func LockCurriculumGraph(tx *sql.Tx) error {
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(781924613)`); err != nil {
		return fmt.Errorf("lock curriculum graph: %w", err)
	}
	return nil
}

func UnitExists(q curriculumExecutor, unitID int64) (bool, error) {
	var exists bool
	if err := q.QueryRow(`SELECT EXISTS (SELECT 1 FROM units WHERE id = $1)`, unitID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check curriculum unit: %w", err)
	}
	return exists, nil
}

func UnitDependentNames(q curriculumExecutor, prerequisiteID int64) ([]string, error) {
	rows, err := q.Query(`
		SELECT unit.name
		FROM unit_dependencies dependency
		JOIN units unit ON unit.id = dependency.unit_id
		WHERE dependency.prerequisite_id = $1
		ORDER BY lower(unit.name), unit.id
	`, prerequisiteID)
	if err != nil {
		return nil, fmt.Errorf("list units requiring prerequisite: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan unit requiring prerequisite: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate units requiring prerequisite: %w", err)
	}
	return names, nil
}

func DeleteUnit(q curriculumExecutor, unitID int64) (bool, error) {
	result, err := q.Exec(`DELETE FROM units WHERE id = $1`, unitID)
	if err != nil {
		return false, fmt.Errorf("delete curriculum unit: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted curriculum units: %w", err)
	}
	return deleted != 0, nil
}

func DependencyCreatesCycle(q curriculumExecutor, unitID, prerequisiteID int64) (bool, error) {
	if unitID == prerequisiteID {
		return true, nil
	}
	var createsCycle bool
	err := q.QueryRow(`
		WITH RECURSIVE prerequisites(id) AS (
			SELECT prerequisite_id
			FROM unit_dependencies
			WHERE unit_id = $1
			UNION
			SELECT dependency.prerequisite_id
			FROM unit_dependencies dependency
			JOIN prerequisites ON dependency.unit_id = prerequisites.id
		)
		SELECT EXISTS (SELECT 1 FROM prerequisites WHERE id = $2)
	`, prerequisiteID, unitID).Scan(&createsCycle)
	if err != nil {
		return false, fmt.Errorf("check unit dependency cycle: %w", err)
	}
	return createsCycle, nil
}

func CreateUnitDependency(q curriculumExecutor, unitID, prerequisiteID int64) (bool, error) {
	result, err := q.Exec(`
		INSERT INTO unit_dependencies (unit_id, prerequisite_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, unitID, prerequisiteID)
	if err != nil {
		return false, fmt.Errorf("create unit dependency: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count created unit dependencies: %w", err)
	}
	return created != 0, nil
}

func DeleteUnitDependency(q curriculumExecutor, unitID, prerequisiteID int64) (bool, error) {
	result, err := q.Exec(`
		DELETE FROM unit_dependencies
		WHERE unit_id = $1 AND prerequisite_id = $2
	`, unitID, prerequisiteID)
	if err != nil {
		return false, fmt.Errorf("delete unit dependency: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted unit dependencies: %w", err)
	}
	return deleted != 0, nil
}
