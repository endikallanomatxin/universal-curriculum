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
	ErrUnitContentRequired       = errors.New("unit content is required")
	ErrDependencyExists          = errors.New("unit dependency already exists")
	ErrDependencyNotFound        = errors.New("unit dependency not found")
	ErrDependencyCycle           = errors.New("unit dependency creates a cycle")
	ErrProposalNotFound          = errors.New("draft curriculum proposal not found")
	ErrProposalTitleRequired     = errors.New("proposal title is required")
	ErrProposalRationaleRequired = errors.New("proposal rationale is required")
	ErrProposalEmpty             = errors.New("curriculum proposal has no changes")
	ErrProposalOutdated          = errors.New("curriculum proposal is not based on the current curriculum")
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
	baseProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return nil, err
	}
	proposal := &models.CurriculumProposal{
		AuthorID: &authorID, Title: title, Rationale: rationale, BaseProposalID: baseProposalID,
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

func CreateCurriculumUnit(database *sql.DB, authorID, proposalID int64, name, content string) (*models.Unit, error) {
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if name == "" {
		return nil, ErrUnitNameRequired
	}
	if content == "" {
		return nil, ErrUnitContentRequired
	}
	unitID, err := db.NextCurriculumUnitID(database)
	if err != nil {
		return nil, err
	}
	change := &models.CurriculumProposalChange{
		Kind: "create_unit", UnitID: unitID, UnitName: name,
		UnitContent: content,
	}
	if err := db.AddDraftCurriculumProposalChange(database, proposalID, authorID, change); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProposalNotFound
		}
		return nil, err
	}
	return &models.Unit{ID: unitID, Name: name, Content: content}, nil
}

func UpdateCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrUnitNameRequired
	}
	created, err := draftCreatedCurriculumUnit(database, authorID, proposalID, unitID)
	if err != nil {
		return err
	}
	if created != nil {
		updated, err := db.UpdateDraftCreatedCurriculumUnit(
			database, proposalID, authorID, unitID, name, created.UnitContent,
		)
		if err != nil {
			return err
		}
		if !updated {
			return ErrProposalNotFound
		}
		return nil
	}
	unit, err := db.GetUnit(database, unitID)
	if err != nil {
		return err
	}
	if unit == nil {
		return ErrUnitNotFound
	}
	change := &models.CurriculumProposalChange{
		Kind: "update_unit", UnitID: unitID, UnitName: name,
		PreviousUnitName: unit.Name,
	}
	if name == unit.Name {
		change = nil
	}
	return replaceUnitProposalChange(database, authorID, proposalID, unitID, "update_unit", change)
}

func UpdateCurriculumUnitContent(database *sql.DB, authorID, proposalID, unitID int64, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrUnitContentRequired
	}
	created, err := draftCreatedCurriculumUnit(database, authorID, proposalID, unitID)
	if err != nil {
		return err
	}
	if created != nil {
		updated, err := db.UpdateDraftCreatedCurriculumUnit(
			database, proposalID, authorID, unitID, created.UnitName, content,
		)
		if err != nil {
			return err
		}
		if !updated {
			return ErrProposalNotFound
		}
		return nil
	}
	unit, err := db.GetUnit(database, unitID)
	if err != nil {
		return err
	}
	if unit == nil {
		return ErrUnitNotFound
	}
	var change *models.CurriculumProposalChange
	if content != unit.Content {
		change = &models.CurriculumProposalChange{
			Kind: "update_content", UnitID: unitID,
			UnitContent: content, PreviousUnitContent: unit.Content,
		}
	}
	return replaceUnitProposalChange(database, authorID, proposalID, unitID, "update_content", change)
}

func draftCreatedCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64) (*models.CurriculumProposalChange, error) {
	proposal, err := db.GetCurriculumProposal(database, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.Status != "draft" || proposal.AuthorID == nil || *proposal.AuthorID != authorID {
		return nil, ErrProposalNotFound
	}
	for index := range proposal.Changes {
		change := &proposal.Changes[index]
		if change.Kind == "create_unit" && change.UnitID == unitID {
			return change, nil
		}
	}
	return nil, nil
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
		Kind: "delete_unit", UnitID: unitID, UnitName: unit.Name,
		UnitContent: unit.Content,
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
	proposal, err := db.GetCurriculumProposal(database, proposalID)
	if err != nil {
		return err
	}
	if proposal == nil || proposal.Status != "draft" || proposal.AuthorID == nil || *proposal.AuthorID != authorID {
		return ErrProposalNotFound
	}
	graph, err := db.GetCurriculumGraph(database)
	if err != nil {
		return err
	}
	workingGraph := CurriculumGraphWithProposal(graph, proposal)
	units := make(map[int64]bool, len(workingGraph.Units))
	deleted := make(map[int64]bool)
	for _, unit := range workingGraph.Units {
		units[unit.ID] = true
	}
	for _, change := range proposal.Changes {
		if change.Kind == "delete_unit" {
			deleted[change.UnitID] = true
		}
	}
	if !units[unitID] || !units[prerequisiteID] || deleted[unitID] || deleted[prerequisiteID] {
		return ErrUnitNotFound
	}
	for _, dependency := range workingGraph.Dependencies {
		if dependency.UnitID == unitID && dependency.PrerequisiteID == prerequisiteID {
			return ErrDependencyExists
		}
	}
	if curriculumDependencyCreatesCycle(workingGraph, unitID, prerequisiteID) {
		return ErrDependencyCycle
	}
	id := prerequisiteID
	return addProposalChange(database, authorID, proposalID, &models.CurriculumProposalChange{
		Kind: "add_dependency", UnitID: unitID, PrerequisiteID: &id,
	})
}

func CurriculumGraphWithProposal(graph *models.CurriculumGraph, proposal *models.CurriculumProposal) *models.CurriculumGraph {
	if graph == nil || proposal == nil {
		return graph
	}
	preview := &models.CurriculumGraph{
		Units:        append([]models.Unit(nil), graph.Units...),
		Dependencies: append([]models.UnitDependency(nil), graph.Dependencies...),
	}
	unitIndexes := make(map[int64]int, len(preview.Units))
	for index := range preview.Units {
		unitIndexes[preview.Units[index].ID] = index
	}
	for _, change := range proposal.Changes {
		switch change.Kind {
		case "create_unit":
			if _, exists := unitIndexes[change.UnitID]; !exists {
				unitIndexes[change.UnitID] = len(preview.Units)
				preview.Units = append(preview.Units, models.Unit{
					ID: change.UnitID, Name: change.UnitName, Content: change.UnitContent,
				})
			}
		case "update_unit":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units[index].Name = change.UnitName
			}
		case "update_content":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units[index].Content = change.UnitContent
			}
		case "add_dependency":
			if change.PrerequisiteID != nil && !curriculumDependencyExists(preview, change.UnitID, *change.PrerequisiteID) {
				preview.Dependencies = append(preview.Dependencies, models.UnitDependency{
					UnitID: change.UnitID, PrerequisiteID: *change.PrerequisiteID,
				})
			}
		case "remove_dependency":
			if change.PrerequisiteID != nil {
				filtered := preview.Dependencies[:0]
				for _, dependency := range preview.Dependencies {
					if dependency.UnitID != change.UnitID || dependency.PrerequisiteID != *change.PrerequisiteID {
						filtered = append(filtered, dependency)
					}
				}
				preview.Dependencies = filtered
			}
		}
	}
	return preview
}

func curriculumDependencyExists(graph *models.CurriculumGraph, unitID, prerequisiteID int64) bool {
	for _, dependency := range graph.Dependencies {
		if dependency.UnitID == unitID && dependency.PrerequisiteID == prerequisiteID {
			return true
		}
	}
	return false
}

func curriculumDependencyCreatesCycle(graph *models.CurriculumGraph, unitID, prerequisiteID int64) bool {
	dependents := make(map[int64][]int64, len(graph.Dependencies))
	for _, dependency := range graph.Dependencies {
		dependents[dependency.PrerequisiteID] = append(dependents[dependency.PrerequisiteID], dependency.UnitID)
	}
	pending := []int64{unitID}
	visited := make(map[int64]bool)
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == prerequisiteID {
			return true
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		pending = append(pending, dependents[current]...)
	}
	return false
}

func RemoveUnitDependency(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64) error {
	exists, err := db.DependencyExists(database, unitID, prerequisiteID)
	if err != nil {
		return err
	}
	exists, err = proposedDependencyExists(database, authorID, proposalID, unitID, prerequisiteID, exists)
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
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return err
	}
	if !sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return ErrProposalOutdated
	}
	ok, err := db.AcceptDraftCurriculumProposal(tx, proposalID, authorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	if err := db.RebuildCurriculumProjection(tx, proposalID); err != nil {
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
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return err
	}
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
		case "update_content":
			change.UnitContent, change.PreviousUnitContent = change.PreviousUnitContent, change.UnitContent
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
	revert := &models.CurriculumProposal{
		AuthorID: &authorID, Title: "Revert: " + target.Title,
		Rationale: "Restore the curriculum state before the previous proposal.",
		Status:    "accepted", BaseProposalID: currentProposalID,
		RevertsProposalID: &target.ID, Changes: changes,
	}
	if err := db.CreateAcceptedCurriculumProposal(tx, revert); err != nil {
		return err
	}
	if err := db.RebuildCurriculumProjection(tx, revert.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum proposal revert: %w", err)
	}
	return nil
}

func sameOptionalID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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

func replaceUnitProposalChange(database *sql.DB, authorID, proposalID, unitID int64, kind string, change *models.CurriculumProposalChange) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	authorized, err := db.DeleteDraftCurriculumProposalUnitChanges(tx, proposalID, authorID, unitID, kind)
	if err != nil {
		return err
	}
	if !authorized {
		return ErrProposalNotFound
	}
	if change != nil {
		if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, change); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrProposalNotFound
			}
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum unit proposal change: %w", err)
	}
	return nil
}

func proposedDependencyExists(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64, published bool) (bool, error) {
	proposal, err := db.GetCurriculumProposal(database, proposalID)
	if err != nil {
		return false, err
	}
	if proposal == nil || proposal.Status != "draft" || proposal.AuthorID == nil || *proposal.AuthorID != authorID {
		return false, ErrProposalNotFound
	}
	exists := published
	for _, change := range proposal.Changes {
		if change.UnitID != unitID || change.PrerequisiteID == nil || *change.PrerequisiteID != prerequisiteID {
			continue
		}
		switch change.Kind {
		case "add_dependency":
			exists = true
		case "remove_dependency":
			exists = false
		}
	}
	return exists, nil
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
