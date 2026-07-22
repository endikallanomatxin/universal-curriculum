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
	ErrUnitNotFound            = errors.New("curriculum unit not found")
	ErrUnitNameRequired        = errors.New("unit name is required")
	ErrUnitDescriptionRequired = errors.New("unit description is required")
	ErrDependencyExists        = errors.New("unit dependency already exists")
	ErrDependencyNotFound      = errors.New("unit dependency not found")
	ErrDependencyCycle         = errors.New("unit dependency creates a cycle")
)

type UnitIsPrerequisiteError struct {
	DependentNames []string
}

func (err *UnitIsPrerequisiteError) Error() string {
	return "unit is required by: " + strings.Join(err.DependentNames, ", ")
}

func CreateCurriculumUnit(database *sql.DB, name, description string) (*models.Unit, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, ErrUnitNameRequired
	}
	if description == "" {
		return nil, ErrUnitDescriptionRequired
	}
	return db.CreateUnit(database, name, description)
}

func DeleteCurriculumUnit(database *sql.DB, unitID int64) error {
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin curriculum unit deletion: %w", err)
	}
	defer tx.Rollback()
	if err := db.LockCurriculumGraph(tx); err != nil {
		return err
	}
	dependentNames, err := db.UnitDependentNames(tx, unitID)
	if err != nil {
		return err
	}
	if len(dependentNames) != 0 {
		return &UnitIsPrerequisiteError{DependentNames: dependentNames}
	}
	deleted, err := db.DeleteUnit(tx, unitID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrUnitNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum unit deletion: %w", err)
	}
	return nil
}

func AddUnitDependency(database *sql.DB, unitID, prerequisiteID int64) error {
	if unitID <= 0 || prerequisiteID <= 0 {
		return ErrUnitNotFound
	}
	if unitID == prerequisiteID {
		return ErrDependencyCycle
	}
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin unit dependency creation: %w", err)
	}
	defer tx.Rollback()
	if err := db.LockCurriculumGraph(tx); err != nil {
		return err
	}
	for _, id := range []int64{unitID, prerequisiteID} {
		exists, err := db.UnitExists(tx, id)
		if err != nil {
			return err
		}
		if !exists {
			return ErrUnitNotFound
		}
	}
	cycle, err := db.DependencyCreatesCycle(tx, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	if cycle {
		return ErrDependencyCycle
	}
	created, err := db.CreateUnitDependency(tx, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	if !created {
		return ErrDependencyExists
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unit dependency creation: %w", err)
	}
	return nil
}

func RemoveUnitDependency(database *sql.DB, unitID, prerequisiteID int64) error {
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin unit dependency deletion: %w", err)
	}
	defer tx.Rollback()
	if err := db.LockCurriculumGraph(tx); err != nil {
		return err
	}
	deleted, err := db.DeleteUnitDependency(tx, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrDependencyNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unit dependency deletion: %w", err)
	}
	return nil
}
