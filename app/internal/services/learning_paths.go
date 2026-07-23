package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

var (
	ErrLearningPathNotFound      = errors.New("learning path not found")
	ErrLearningPathNameRequired  = errors.New("learning path name is required")
	ErrLearningPathUnitsRequired = errors.New("learning path requires at least one target unit")
)

func CreateLearningPath(database *sql.DB, userID int64, name, description string, unitIDs []int64) (*models.LearningPath, error) {
	path, unitIDs, err := validatedLearningPath(database, userID, 0, name, description, unitIDs)
	if err != nil {
		return nil, err
	}
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin learning path creation: %w", err)
	}
	defer tx.Rollback()
	if err := db.InsertLearningPath(tx, path); err != nil {
		return nil, err
	}
	if err := db.ReplaceLearningPathUnits(tx, path.ID, unitIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit learning path creation: %w", err)
	}
	return path, nil
}

func UpdateLearningPath(database *sql.DB, userID, pathID int64, name, description string, unitIDs []int64) error {
	path, unitIDs, err := validatedLearningPath(database, userID, pathID, name, description, unitIDs)
	if err != nil {
		return err
	}
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin learning path update: %w", err)
	}
	defer tx.Rollback()
	ok, err := db.UpdateLearningPath(tx, path)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLearningPathNotFound
	}
	if err := db.ReplaceLearningPathUnits(tx, path.ID, unitIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit learning path update: %w", err)
	}
	return nil
}

func validatedLearningPath(
	database *sql.DB,
	userID, pathID int64,
	name, description string,
	unitIDs []int64,
) (*models.LearningPath, []int64, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if name == "" {
		return nil, nil, ErrLearningPathNameRequired
	}
	seen := make(map[int64]bool, len(unitIDs))
	validated := make([]int64, 0, len(unitIDs))
	for _, unitID := range unitIDs {
		if unitID <= 0 || seen[unitID] {
			continue
		}
		unit, err := db.GetUnit(database, unitID)
		if err != nil {
			return nil, nil, err
		}
		if unit == nil {
			return nil, nil, ErrUnitNotFound
		}
		seen[unitID] = true
		validated = append(validated, unitID)
	}
	if len(validated) == 0 {
		return nil, nil, ErrLearningPathUnitsRequired
	}
	return &models.LearningPath{
		ID: pathID, UserID: userID, Name: name, Description: description,
	}, validated, nil
}

func CurriculumPathSubgraph(graph *models.CurriculumGraph, targetIDs []int64) *models.CurriculumGraph {
	subgraph := &models.CurriculumGraph{}
	if graph == nil {
		return subgraph
	}
	units := make(map[int64]models.Unit, len(graph.Units))
	prerequisites := make(map[int64][]int64)
	for _, unit := range graph.Units {
		units[unit.ID] = unit
	}
	for _, dependency := range graph.Dependencies {
		prerequisites[dependency.UnitID] = append(prerequisites[dependency.UnitID], dependency.PrerequisiteID)
	}
	included := make(map[int64]bool)
	pending := append([]int64(nil), targetIDs...)
	for len(pending) > 0 {
		unitID := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if included[unitID] {
			continue
		}
		if _, exists := units[unitID]; !exists {
			continue
		}
		included[unitID] = true
		pending = append(pending, prerequisites[unitID]...)
	}
	for _, unit := range graph.Units {
		if included[unit.ID] {
			subgraph.Units = append(subgraph.Units, unit)
		}
	}
	for _, dependency := range graph.Dependencies {
		if included[dependency.UnitID] && included[dependency.PrerequisiteID] {
			subgraph.Dependencies = append(subgraph.Dependencies, dependency)
		}
	}
	return subgraph
}
