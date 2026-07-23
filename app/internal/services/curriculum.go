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
	ErrUnitNotFound              = errors.New("curriculum unit not found")
	ErrUnitNameRequired          = errors.New("unit name is required")
	ErrUnitDescriptionRequired   = errors.New("unit description is required")
	ErrDependencyExists          = errors.New("unit dependency already exists")
	ErrDependencyNotFound        = errors.New("unit dependency not found")
	ErrDependencyCycle           = errors.New("unit dependency creates a cycle")
	ErrProposalNotFound          = errors.New("draft curriculum proposal not found")
	ErrProposalTitleRequired     = errors.New("proposal title is required")
	ErrProposalRationaleRequired = errors.New("proposal rationale is required")
	ErrProposalEmpty             = errors.New("curriculum proposal has no changes")
	ErrProposalOutdated          = errors.New("curriculum proposal is based on an old version")
	ErrNoProposalToRevert        = errors.New("there is no curriculum proposal to revert")
)

type UnitIsPrerequisiteError struct{ DependentNames []string }

func (err *UnitIsPrerequisiteError) Error() string {
	return "unit is required by: " + strings.Join(err.DependentNames, ", ")
}

func CreateCurriculumProposal(database *sql.DB, authorID int64, title, rationale string) (*models.CurriculumProposal, error) {
	title, rationale, err := validateProposalMetadata(title, rationale)
	if err != nil {
		return nil, err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	version, err := db.CurrentCurriculumVersion(tx)
	if err != nil {
		return nil, err
	}
	proposal := &models.CurriculumProposal{
		AuthorID: &authorID, Title: title, Rationale: rationale, BaseVersion: version,
	}
	if err := db.CreateDraftCurriculumProposal(tx, proposal); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit draft curriculum proposal: %w", err)
	}
	return proposal, nil
}

func UpdateCurriculumProposal(database *sql.DB, authorID, proposalID int64, title, rationale string) error {
	title, rationale, err := validateProposalMetadata(title, rationale)
	if err != nil {
		return err
	}
	ok, err := db.UpdateDraftCurriculumProposal(database, proposalID, authorID, title, rationale)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	return nil
}

func DeleteCurriculumProposal(database *sql.DB, authorID, proposalID int64) error {
	ok, err := db.DeleteDraftCurriculumProposal(database, proposalID, authorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	return nil
}

func CreateCurriculumUnit(database *sql.DB, authorID, proposalID int64, name, description string) (*models.Unit, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, ErrUnitNameRequired
	}
	if description == "" {
		return nil, ErrUnitDescriptionRequired
	}
	unitID, err := db.NextCurriculumUnitID(database)
	if err != nil {
		return nil, err
	}
	change := &models.CurriculumProposalChange{
		Kind: "create_unit", UnitID: unitID, UnitName: name, UnitDescription: description,
	}
	if err := db.AddDraftCurriculumProposalChange(database, proposalID, authorID, change); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProposalNotFound
		}
		return nil, err
	}
	return &models.Unit{ID: unitID, Name: name, Description: description}, nil
}

func UpdateCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64, name, description string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return ErrUnitNameRequired
	}
	if description == "" {
		return ErrUnitDescriptionRequired
	}
	unit, err := db.GetUnit(database, unitID)
	if err != nil {
		return err
	}
	if unit == nil {
		return ErrUnitNotFound
	}
	change := &models.CurriculumProposalChange{
		Kind: "update_unit", UnitID: unitID, UnitName: name, UnitDescription: description,
		PreviousUnitName: unit.Name, PreviousUnitDescription: unit.Description,
	}
	return addProposalChange(database, authorID, proposalID, change)
}

func DeleteCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64) error {
	unit, err := db.GetUnit(database, unitID)
	if err != nil {
		return err
	}
	if unit == nil {
		return ErrUnitNotFound
	}
	change := &models.CurriculumProposalChange{
		Kind: "delete_unit", UnitID: unitID, UnitName: unit.Name, UnitDescription: unit.Description,
	}
	return addProposalChange(database, authorID, proposalID, change)
}

func AddUnitDependency(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64) error {
	if unitID <= 0 || prerequisiteID <= 0 {
		return ErrUnitNotFound
	}
	if unitID == prerequisiteID {
		return ErrDependencyCycle
	}
	for _, id := range []int64{unitID, prerequisiteID} {
		unit, err := db.GetUnit(database, id)
		if err != nil {
			return err
		}
		if unit == nil {
			return ErrUnitNotFound
		}
	}
	cycle, err := db.DependencyCreatesCycle(database, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	if cycle {
		return ErrDependencyCycle
	}
	exists, err := db.DependencyExists(database, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDependencyExists
	}
	id := prerequisiteID
	return addProposalChange(database, authorID, proposalID, &models.CurriculumProposalChange{
		Kind: "add_dependency", UnitID: unitID, PrerequisiteID: &id,
	})
}

func RemoveUnitDependency(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64) error {
	exists, err := db.DependencyExists(database, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDependencyNotFound
	}
	id := prerequisiteID
	return addProposalChange(database, authorID, proposalID, &models.CurriculumProposalChange{
		Kind: "remove_dependency", UnitID: unitID, PrerequisiteID: &id,
	})
}

func DeleteCurriculumProposalChange(database *sql.DB, authorID, proposalID, changeID int64) error {
	ok, err := db.DeleteDraftCurriculumProposalChange(database, proposalID, changeID, authorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	return nil
}

func PublishCurriculumProposal(database *sql.DB, authorID, proposalID int64) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := db.GetCurriculumProposal(tx, proposalID)
	if err != nil {
		return err
	}
	if proposal == nil || proposal.Status != "draft" || proposal.AuthorID == nil || *proposal.AuthorID != authorID {
		return ErrProposalNotFound
	}
	if len(proposal.Changes) == 0 {
		return ErrProposalEmpty
	}
	version, err := db.CurrentCurriculumVersion(tx)
	if err != nil {
		return err
	}
	if proposal.BaseVersion != version {
		return ErrProposalOutdated
	}
	publishedVersion := version + 1
	ok, err := db.AcceptDraftCurriculumProposal(tx, proposalID, authorID, publishedVersion)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	if err := db.RebuildCurriculumProjection(tx, publishedVersion, proposalID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum proposal publication: %w", err)
	}
	return nil
}

func RevertCurriculumProposal(database *sql.DB, authorID, proposalID int64) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	target, err := db.GetLatestRevertibleCurriculumProposal(tx)
	if err != nil {
		return err
	}
	if target == nil {
		return ErrNoProposalToRevert
	}
	if target.ID != proposalID {
		return ErrNoProposalToRevert
	}
	changes := make([]models.CurriculumProposalChange, 0, len(target.Changes))
	for index := len(target.Changes) - 1; index >= 0; index-- {
		change := target.Changes[index]
		switch change.Kind {
		case "create_unit":
			change.Kind = "delete_unit"
		case "delete_unit":
			change.Kind = "create_unit"
		case "update_unit":
			change.UnitName, change.PreviousUnitName = change.PreviousUnitName, change.UnitName
			change.UnitDescription, change.PreviousUnitDescription = change.PreviousUnitDescription, change.UnitDescription
		case "add_dependency":
			change.Kind = "remove_dependency"
		case "remove_dependency":
			change.Kind = "add_dependency"
		default:
			return fmt.Errorf("revert unsupported curriculum change %q", change.Kind)
		}
		change.ID, change.ProposalID, change.Position = 0, 0, 0
		changes = append(changes, change)
	}
	version, err := db.CurrentCurriculumVersion(tx)
	if err != nil {
		return err
	}
	publishedVersion := version + 1
	revert := &models.CurriculumProposal{
		AuthorID: &authorID, Title: "Revert: " + target.Title,
		Rationale: "Restore the curriculum state before the previous proposal.",
		Status:    "accepted", BaseVersion: version, PublishedVersion: &publishedVersion,
		RevertsProposalID: &target.ID, Changes: changes,
	}
	if err := db.CreateAcceptedCurriculumProposal(tx, revert); err != nil {
		return err
	}
	if err := db.RebuildCurriculumProjection(tx, publishedVersion, revert.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum proposal revert: %w", err)
	}
	return nil
}

func validateProposalMetadata(title, rationale string) (string, string, error) {
	title, rationale = strings.TrimSpace(title), strings.TrimSpace(rationale)
	if title == "" {
		return "", "", ErrProposalTitleRequired
	}
	if rationale == "" {
		return "", "", ErrProposalRationaleRequired
	}
	return title, rationale, nil
}

func addProposalChange(database *sql.DB, authorID, proposalID int64, change *models.CurriculumProposalChange) error {
	if err := db.AddDraftCurriculumProposalChange(database, proposalID, authorID, change); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProposalNotFound
		}
		return err
	}
	return nil
}

func beginCurriculumProposal(database *sql.DB) (*sql.Tx, error) {
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin curriculum proposal: %w", err)
	}
	if err := db.LockCurriculumGraph(tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}
