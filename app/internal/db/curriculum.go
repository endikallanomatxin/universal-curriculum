package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"universal-curriculum/internal/models"
)

type curriculumExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type contextualCurriculumExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type curriculumExecutorWithContext struct {
	context context.Context
	q       contextualCurriculumExecutor
}

// WithContext binds existing database operations to the lifecycle of a caller.
func WithContext(ctx context.Context, q contextualCurriculumExecutor) curriculumExecutor {
	return curriculumExecutorWithContext{context: ctx, q: q}
}

func (q curriculumExecutorWithContext) Exec(query string, args ...any) (sql.Result, error) {
	return q.q.ExecContext(q.context, query, args...)
}

func (q curriculumExecutorWithContext) Query(query string, args ...any) (*sql.Rows, error) {
	return q.q.QueryContext(q.context, query, args...)
}

func (q curriculumExecutorWithContext) QueryRow(query string, args ...any) *sql.Row {
	return q.q.QueryRowContext(q.context, query, args...)
}

func GetCurriculumGraph(database curriculumExecutor) (*models.CurriculumGraph, error) {
	return getCurriculumGraph(database, false)
}

func GetCurriculumGraphWithContent(database curriculumExecutor) (*models.CurriculumGraph, error) {
	return getCurriculumGraph(database, true)
}

func getCurriculumGraph(database curriculumExecutor, includeContent bool) (*models.CurriculumGraph, error) {
	graph := &models.CurriculumGraph{}
	query := `SELECT id, name FROM units ORDER BY lower(name), id`
	if includeContent {
		query = `SELECT id, name, content, created_at, updated_at FROM units ORDER BY lower(name), id`
	}
	rows, err := database.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list curriculum units: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var unit models.Unit
		var scanErr error
		if includeContent {
			scanErr = rows.Scan(&unit.ID, &unit.Name, &unit.Content, &unit.CreatedAt, &unit.UpdatedAt)
		} else {
			scanErr = rows.Scan(&unit.ID, &unit.Name)
		}
		if scanErr != nil {
			return nil, fmt.Errorf("scan curriculum unit: %w", scanErr)
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

func GetUnit(q curriculumExecutor, unitID int64) (*models.Unit, error) {
	var unit models.Unit
	err := q.QueryRow(`
		SELECT id, name, content, created_at, updated_at
		FROM units
		WHERE id = $1
	`, unitID).Scan(&unit.ID, &unit.Name, &unit.Content, &unit.CreatedAt, &unit.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get curriculum unit: %w", err)
	}
	return &unit, nil
}

func GetUnitDependencies(q curriculumExecutor, unitID int64) ([]models.UnitDependency, error) {
	rows, err := q.Query(`
		SELECT unit_id, prerequisite_id
		FROM unit_dependencies
		WHERE unit_id = $1 OR prerequisite_id = $1
		ORDER BY unit_id, prerequisite_id
	`, unitID)
	if err != nil {
		return nil, fmt.Errorf("list curriculum unit dependencies: %w", err)
	}
	defer rows.Close()
	var dependencies []models.UnitDependency
	for rows.Next() {
		var dependency models.UnitDependency
		if err := rows.Scan(&dependency.UnitID, &dependency.PrerequisiteID); err != nil {
			return nil, fmt.Errorf("scan curriculum unit dependency: %w", err)
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate curriculum unit dependencies: %w", err)
	}
	return dependencies, nil
}

func SearchCurriculumUnits(
	q curriculumExecutor, query string, limit, offset int,
) ([]models.Unit, int, error) {
	query = strings.TrimSpace(query)
	rows, err := q.Query(`
		SELECT id, name, count(*) OVER ()
		FROM units
		WHERE $1 = '' OR strpos(lower(name), lower($1)) > 0 OR strpos(lower(content), lower($1)) > 0
		ORDER BY lower(name), id
		LIMIT $2 OFFSET $3
	`, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search curriculum units: %w", err)
	}
	defer rows.Close()
	units := make([]models.Unit, 0)
	var total int
	for rows.Next() {
		var unit models.Unit
		if err := rows.Scan(&unit.ID, &unit.Name, &total); err != nil {
			return nil, 0, fmt.Errorf("scan curriculum unit summary: %w", err)
		}
		units = append(units, unit)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate curriculum unit summaries: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, fmt.Errorf("close curriculum unit summaries: %w", err)
	}
	if len(units) == 0 {
		if err := q.QueryRow(`
			SELECT count(*)
			FROM units
			WHERE $1 = '' OR strpos(lower(name), lower($1)) > 0 OR strpos(lower(content), lower($1)) > 0
		`, query).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count matching curriculum units: %w", err)
		}
	}
	return units, total, nil
}

func SearchCurriculumUnitNames(
	ctx context.Context, q contextualCurriculumExecutor, query string, limit int,
) ([]models.Unit, error) {
	return searchCurriculumUnitNames(ctx, q, `
		SELECT id, name
		FROM units
		WHERE strpos(lower(name), lower($1)) > 0
		ORDER BY lower(name), id
		LIMIT $2
	`, query, limit)
}

func SearchDraftCurriculumUnitNames(
	ctx context.Context, q contextualCurriculumExecutor, proposalID int64, query string, limit int,
) ([]models.Unit, error) {
	return searchCurriculumUnitNames(ctx, q, `
		WITH candidates AS (
			SELECT unit.id,
			       COALESCE(rename.unit_name, unit.name) AS name
			FROM units unit
			LEFT JOIN curriculum_proposal_change_details rename
			  ON rename.proposal_id = $3
			 AND rename.kind = 'rename_unit'
			 AND rename.unit_id = unit.id
			WHERE NOT EXISTS (
				SELECT 1
				FROM curriculum_proposal_change_details deletion
				WHERE deletion.proposal_id = $3
				  AND deletion.kind = 'delete_unit'
				  AND deletion.unit_id = unit.id
			)
			UNION ALL
			SELECT creation.unit_id, creation.unit_name
			FROM curriculum_proposal_change_details creation
			WHERE creation.proposal_id = $3
			  AND creation.kind = 'create_unit'
		)
		SELECT id, name
		FROM candidates
		WHERE strpos(lower(name), lower($1)) > 0
		ORDER BY lower(name), id
		LIMIT $2
	`, query, limit, proposalID)
}

func searchCurriculumUnitNames(
	ctx context.Context,
	q contextualCurriculumExecutor,
	sqlQuery string,
	query string,
	limit int,
	extra ...any,
) ([]models.Unit, error) {
	args := []any{query, limit}
	args = append(args, extra...)
	rows, err := q.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search curriculum unit names: %w", err)
	}
	defer rows.Close()
	var units []models.Unit
	for rows.Next() {
		var unit models.Unit
		if err := rows.Scan(&unit.ID, &unit.Name); err != nil {
			return nil, fmt.Errorf("scan curriculum unit name search: %w", err)
		}
		units = append(units, unit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate curriculum unit name search: %w", err)
	}
	return units, nil
}
