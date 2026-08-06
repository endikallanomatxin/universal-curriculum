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
	ErrUnitNotFound               = errors.New("curriculum unit not found")
	ErrUnitNameRequired           = errors.New("unit name is required")
	ErrUnitContentRequired        = errors.New("unit content is required")
	ErrDependencyExists           = errors.New("unit dependency already exists")
	ErrDependencyNotFound         = errors.New("unit dependency not found")
	ErrDependencyCycle            = errors.New("unit dependency creates a cycle")
	ErrProposalNotFound           = errors.New("draft curriculum proposal not found")
	ErrProposalTitleRequired      = errors.New("proposal title is required")
	ErrProposalRationaleRequired  = errors.New("proposal rationale is required")
	ErrProposalEmpty              = errors.New("curriculum proposal has no changes")
	ErrProposalOutdated           = errors.New("curriculum proposal is not based on the current curriculum")
	ErrRecognitionSourcesRequired = errors.New("recognition requires at least one source")
	ErrRecognitionTargetsRequired = errors.New("recognition requires at least one target")
)

type UnitIsPrerequisiteError struct{ DependentNames []string }

func (err *UnitIsPrerequisiteError) Error() string {
	return "unit is required by: " + strings.Join(err.DependentNames, ", ")
}

type RecognitionCoverageWarning struct {
	CreatedWithoutSource []models.Unit
	DeletedWithoutTarget []models.Unit
}

func CurriculumRecognitionCoverage(
	proposal *models.CurriculumProposal,
) RecognitionCoverageWarning {
	var warning RecognitionCoverageWarning
	if proposal == nil {
		return warning
	}
	incoming := make(map[int64]bool)
	outgoing := make(map[int64]bool)
	for _, change := range proposal.Changes {
		if change.Recognition == nil {
			continue
		}
		for _, source := range change.Recognition.Sources {
			outgoing[source.ID] = true
		}
		for _, target := range change.Recognition.Targets {
			incoming[target.ID] = true
		}
	}
	for _, change := range proposal.Changes {
		switch change.Kind {
		case "create_unit":
			if !incoming[change.UnitID] {
				warning.CreatedWithoutSource = append(warning.CreatedWithoutSource, models.Unit{
					ID: change.UnitID, Name: change.UnitName,
				})
			}
		case "delete_unit":
			if !outgoing[change.UnitID] {
				warning.DeletedWithoutTarget = append(warning.DeletedWithoutTarget, models.Unit{
					ID: change.UnitID, Name: change.UnitName,
				})
			}
		}
	}
	return warning
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
		AuthorIDs: []int64{authorID}, Title: title, Rationale: rationale, BaseProposalID: baseProposalID,
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
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return nil, err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := currentDraftCurriculumProposal(tx, authorID, proposalID); err != nil {
		return nil, err
	}
	change := &models.CurriculumProposalChange{
		Kind: "create_unit", UnitName: name, UnitContent: content,
	}
	if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, change); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProposalNotFound
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit curriculum unit creation: %w", err)
	}
	return &models.Unit{ID: change.UnitID, Name: name, Content: content}, nil
}

func UpdateCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrUnitNameRequired
	}
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	created, err := draftCreatedCurriculumUnit(database, authorID, proposalID, unitID)
	if err != nil {
		return err
	}
	if created != nil {
		var change *models.CurriculumProposalChange
		if name != created.UnitName {
			change = &models.CurriculumProposalChange{
				Kind: "rename_unit", UnitID: unitID, UnitName: name,
			}
		}
		return replaceUnitProposalChange(database, authorID, proposalID, unitID, "rename_unit", change)
	}
	unit, err := db.GetUnit(database, unitID)
	if err != nil {
		return err
	}
	if unit == nil {
		return ErrUnitNotFound
	}
	change := &models.CurriculumProposalChange{
		Kind: "rename_unit", UnitID: unitID, UnitName: name,
	}
	if name == unit.Name {
		change = nil
	}
	return replaceUnitProposalChange(database, authorID, proposalID, unitID, "rename_unit", change)
}

func UpdateCurriculumUnitContent(database *sql.DB, authorID, proposalID, unitID int64, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrUnitContentRequired
	}
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	created, err := draftCreatedCurriculumUnit(database, authorID, proposalID, unitID)
	if err != nil {
		return err
	}
	if created != nil {
		var change *models.CurriculumProposalChange
		if content != created.UnitContent {
			change = &models.CurriculumProposalChange{
				Kind: "update_content", UnitID: unitID,
				UnitContent: content,
			}
		}
		return replaceUnitProposalChange(database, authorID, proposalID, unitID, "update_content", change)
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
			UnitContent: content,
		}
	}
	return replaceUnitProposalChange(database, authorID, proposalID, unitID, "update_content", change)
}

// UpdateCurriculumUnitAndContent replaces both editable fields in one
// transaction. It is used by representations that submit a complete unit
// rather than the web interface's independent inline fields.
func UpdateCurriculumUnitAndContent(
	database *sql.DB, authorID, proposalID, unitID int64, name, content string,
) error {
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if name == "" {
		return ErrUnitNameRequired
	}
	if content == "" {
		return ErrUnitContentRequired
	}
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := currentDraftCurriculumProposal(tx, authorID, proposalID)
	if err != nil {
		return err
	}
	var baseline *models.Unit
	for _, change := range proposal.Changes {
		if change.Kind == "delete_unit" && change.UnitID == unitID {
			return ErrUnitNotFound
		}
		if change.Kind == "create_unit" && change.UnitID == unitID {
			baseline = &models.Unit{ID: unitID, Name: change.UnitName, Content: change.UnitContent}
		}
	}
	if baseline == nil {
		baseline, err = db.GetUnit(tx, unitID)
		if err != nil {
			return err
		}
		if baseline == nil {
			return ErrUnitNotFound
		}
	}
	for _, kind := range []string{"rename_unit", "update_content"} {
		authorized, err := db.DeleteDraftCurriculumProposalUnitChanges(
			tx, proposalID, authorID, unitID, kind,
		)
		if err != nil {
			return err
		}
		if !authorized {
			return ErrProposalNotFound
		}
	}
	if name != baseline.Name {
		if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, &models.CurriculumProposalChange{
			Kind: "rename_unit", UnitID: unitID, UnitName: name,
		}); err != nil {
			return err
		}
	}
	if content != baseline.Content {
		if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, &models.CurriculumProposalChange{
			Kind: "update_content", UnitID: unitID, UnitContent: content,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum unit replacement: %w", err)
	}
	return nil
}

func draftCreatedCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64) (*models.CurriculumProposalChange, error) {
	proposal, err := db.GetCurriculumProposal(database, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.Status != "draft" || !proposal.HasAuthor(authorID) {
		return nil, ErrProposalNotFound
	}
	for index := range proposal.Changes {
		change := &proposal.Changes[index]
		if change.Kind == "delete_unit" && change.UnitID == unitID {
			return nil, ErrUnitNotFound
		}
		if change.Kind == "create_unit" && change.UnitID == unitID {
			return change, nil
		}
	}
	return nil, nil
}

func DeleteCurriculumUnit(database *sql.DB, authorID, proposalID, unitID int64) error {
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := currentDraftCurriculumProposal(tx, authorID, proposalID)
	if err != nil {
		return err
	}
	for _, change := range proposal.Changes {
		if change.Kind == "create_unit" && change.UnitID == unitID {
			if err := deleteDraftCreatedUnit(tx, proposal, authorID, change); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit proposed curriculum unit discard: %w", err)
			}
			return nil
		}
	}
	for _, change := range proposal.Changes {
		if change.Kind == "delete_unit" && change.UnitID == unitID {
			return ErrUnitNotFound
		}
	}
	graph, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return err
	}
	workingGraph := CurriculumGraphWithProposal(graph, proposal)
	var unit *models.Unit
	for index := range workingGraph.Units {
		if workingGraph.Units[index].ID == unitID {
			unit = &workingGraph.Units[index]
			break
		}
	}
	if unit == nil {
		return ErrUnitNotFound
	}
	var dependentNames []string
	for _, dependency := range workingGraph.Dependencies {
		if dependency.PrerequisiteID == unitID {
			dependentNames = append(dependentNames, dependency.UnitName)
		}
	}
	if len(dependentNames) > 0 {
		return &UnitIsPrerequisiteError{DependentNames: dependentNames}
	}
	if curriculumUnitByID(graph, unitID) == nil {
		return ErrUnitNotFound
	}
	for _, change := range proposal.Changes {
		supersededUnitChange := change.UnitID == unitID &&
			(change.Kind == "rename_unit" || change.Kind == "update_content")
		supersededOutgoingDependency := change.UnitID == unitID &&
			(change.Kind == "add_dependency" || change.Kind == "remove_dependency")
		supersededRecognition := recognitionContainsUnit(
			change.Recognition, unitID, false, true,
		)
		if supersededUnitChange || supersededOutgoingDependency || supersededRecognition {
			if _, err := db.DeleteDraftCurriculumProposalChange(tx, proposalID, change.ID, authorID); err != nil {
				return err
			}
		}
	}
	if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, &models.CurriculumProposalChange{
		Kind: "delete_unit", UnitID: unitID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProposalNotFound
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum unit deletion: %w", err)
	}
	return nil
}

func deleteDraftCreatedUnit(tx *sql.Tx, proposal *models.CurriculumProposal, authorID int64, creation models.CurriculumProposalChange) error {
	for _, change := range proposal.Changes {
		referencesUnit := change.UnitID == creation.UnitID ||
			change.PrerequisiteID != nil && *change.PrerequisiteID == creation.UnitID ||
			recognitionContainsUnit(change.Recognition, creation.UnitID, true, true)
		if change.ID == creation.ID || !referencesUnit {
			continue
		}
		if _, err := db.DeleteDraftCurriculumProposalChange(tx, proposal.ID, change.ID, authorID); err != nil {
			return err
		}
	}
	deleted, err := db.DeleteDraftCurriculumProposalChange(tx, proposal.ID, creation.ID, authorID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrProposalNotFound
	}
	return nil
}

func AddCurriculumRecognition(
	database *sql.DB,
	authorID, proposalID int64,
	sourceUnitIDs, targetUnitIDs []int64,
) error {
	sourceUnitIDs = uniquePositiveIDs(sourceUnitIDs)
	targetUnitIDs = uniquePositiveIDs(targetUnitIDs)
	if len(sourceUnitIDs) == 0 {
		return ErrRecognitionSourcesRequired
	}
	if len(targetUnitIDs) == 0 {
		return ErrRecognitionTargetsRequired
	}
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := currentDraftCurriculumProposal(tx, authorID, proposalID)
	if err != nil {
		return err
	}
	base, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return err
	}
	result := CurriculumGraphWithProposal(base, proposal)
	recognition := &models.Recognition{}
	for _, unitID := range sourceUnitIDs {
		unit := curriculumUnitByID(base, unitID)
		if unit == nil {
			return ErrUnitNotFound
		}
		recognition.Sources = append(recognition.Sources, *unit)
	}
	for _, unitID := range targetUnitIDs {
		unit := curriculumUnitByID(result, unitID)
		if unit == nil {
			return ErrUnitNotFound
		}
		recognition.Targets = append(recognition.Targets, *unit)
	}
	if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, &models.CurriculumProposalChange{
		Kind: "recognition", Recognition: recognition,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProposalNotFound
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum recognition: %w", err)
	}
	return nil
}

func uniquePositiveIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	return unique
}

func recognitionContainsUnit(
	recognition *models.Recognition,
	unitID int64,
	includeSources, includeTargets bool,
) bool {
	if recognition == nil {
		return false
	}
	if includeSources {
		for _, source := range recognition.Sources {
			if source.ID == unitID {
				return true
			}
		}
	}
	if includeTargets {
		for _, target := range recognition.Targets {
			if target.ID == unitID {
				return true
			}
		}
	}
	return false
}

func AddUnitDependency(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64) error {
	if unitID <= 0 || prerequisiteID <= 0 {
		return ErrUnitNotFound
	}
	if unitID == prerequisiteID {
		return ErrDependencyCycle
	}
	return setUnitDependency(database, authorID, proposalID, unitID, prerequisiteID, true)
}

func RemoveUnitDependency(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64) error {
	if unitID <= 0 || prerequisiteID <= 0 {
		return ErrUnitNotFound
	}
	return setUnitDependency(database, authorID, proposalID, unitID, prerequisiteID, false)
}

func setUnitDependency(database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64, desired bool) error {
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := currentDraftCurriculumProposal(tx, authorID, proposalID)
	if err != nil {
		return err
	}
	graph, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return err
	}
	workingGraph := CurriculumGraphWithProposal(graph, proposal)
	if curriculumUnitByID(workingGraph, unitID) == nil || curriculumUnitByID(workingGraph, prerequisiteID) == nil {
		return ErrUnitNotFound
	}
	exists := curriculumDependencyExists(workingGraph, unitID, prerequisiteID)
	if desired {
		if exists {
			return ErrDependencyExists
		}
		if curriculumDependencyCreatesCycle(workingGraph, unitID, prerequisiteID) {
			return ErrDependencyCycle
		}
	} else if !exists {
		return ErrDependencyNotFound
	}
	for _, change := range proposal.Changes {
		if change.PrerequisiteID == nil ||
			change.UnitID != unitID ||
			*change.PrerequisiteID != prerequisiteID ||
			(change.Kind != "add_dependency" && change.Kind != "remove_dependency") {
			continue
		}
		if _, err := db.DeleteDraftCurriculumProposalChange(tx, proposalID, change.ID, authorID); err != nil {
			return err
		}
	}
	if desired != curriculumDependencyExists(graph, unitID, prerequisiteID) {
		kind := "remove_dependency"
		if desired {
			kind = "add_dependency"
		}
		id := prerequisiteID
		if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, &models.CurriculumProposalChange{
			Kind: kind, UnitID: unitID, PrerequisiteID: &id,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrProposalNotFound
			}
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum dependency change: %w", err)
	}
	return nil
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
	for _, change := range canonicalCurriculumProposalChanges(proposal.Changes) {
		switch change.Kind {
		case "create_unit":
			if _, exists := unitIndexes[change.UnitID]; !exists {
				unitIndexes[change.UnitID] = len(preview.Units)
				preview.Units = append(preview.Units, models.Unit{
					ID: change.UnitID, Name: change.UnitName, Content: change.UnitContent,
				})
			}
		case "rename_unit":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units[index].Name = change.UnitName
			}
		case "update_content":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units[index].Content = change.UnitContent
			}
		case "delete_unit":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units = append(preview.Units[:index], preview.Units[index+1:]...)
				delete(unitIndexes, change.UnitID)
				for following := index; following < len(preview.Units); following++ {
					unitIndexes[preview.Units[following].ID] = following
				}
			}
			filtered := preview.Dependencies[:0]
			for _, dependency := range preview.Dependencies {
				if dependency.UnitID != change.UnitID && dependency.PrerequisiteID != change.UnitID {
					filtered = append(filtered, dependency)
				}
			}
			preview.Dependencies = filtered
		case "add_dependency":
			if change.PrerequisiteID != nil && !curriculumDependencyExists(preview, change.UnitID, *change.PrerequisiteID) {
				dependency := models.UnitDependency{
					UnitID: change.UnitID, PrerequisiteID: *change.PrerequisiteID,
				}
				if index, exists := unitIndexes[change.UnitID]; exists {
					dependency.UnitName = preview.Units[index].Name
				}
				if index, exists := unitIndexes[*change.PrerequisiteID]; exists {
					dependency.PrerequisiteName = preview.Units[index].Name
				}
				preview.Dependencies = append(preview.Dependencies, dependency)
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

func curriculumUnitByID(graph *models.CurriculumGraph, unitID int64) *models.Unit {
	for index := range graph.Units {
		if graph.Units[index].ID == unitID {
			return &graph.Units[index]
		}
	}
	return nil
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

func DeleteCurriculumProposalChange(database *sql.DB, authorID, proposalID, changeID int64) error {
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := currentDraftCurriculumProposal(tx, authorID, proposalID)
	if err != nil {
		return err
	}
	var target *models.CurriculumProposalChange
	for index := range proposal.Changes {
		if proposal.Changes[index].ID == changeID {
			target = &proposal.Changes[index]
			break
		}
	}
	if target == nil {
		return ErrProposalNotFound
	}
	if target.Kind == "create_unit" {
		if err := deleteDraftCreatedUnit(tx, proposal, authorID, *target); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit curriculum proposal change deletion: %w", err)
		}
		return nil
	}
	ok, err := db.DeleteDraftCurriculumProposalChange(tx, proposalID, changeID, authorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum proposal change deletion: %w", err)
	}
	return nil
}

func PublishCurriculumProposal(
	database *sql.DB,
	authorID, proposalID int64,
) (CurriculumProposalRebaseSummary, error) {
	var summary CurriculumProposalRebaseSummary
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return summary, err
	}
	defer tx.Rollback()
	proposal, err := db.GetCurriculumProposal(tx, proposalID)
	if err != nil {
		return summary, err
	}
	if proposal == nil || proposal.Status != "draft" || !proposal.HasAuthor(authorID) {
		return summary, ErrProposalNotFound
	}
	if len(proposal.Changes) == 0 {
		return summary, ErrProposalEmpty
	}
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return summary, err
	}
	graph, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return summary, err
	}
	if !sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		plan, err := planCurriculumProposalRebase(tx, proposal, currentProposalID, graph)
		if err != nil {
			return summary, err
		}
		if plan.Status != ProposalRebaseAutomatic {
			return summary, ErrProposalRebaseRequired
		}
		updated, err := db.SetDraftCurriculumProposalBase(
			tx, proposal.ID, proposal.BaseProposalID, currentProposalID,
		)
		if err != nil {
			return summary, err
		}
		if !updated {
			return summary, ErrProposalOutdated
		}
		proposal.BaseProposalID = currentProposalID
	}
	if err := validateCurriculumProposal(graph, proposal); err != nil {
		return summary, err
	}
	ok, err := db.AcceptDraftCurriculumProposal(tx, proposalID, authorID)
	if err != nil {
		return summary, err
	}
	if !ok {
		return summary, ErrProposalNotFound
	}
	if err := db.MaterializeCurriculumRecognitions(tx, proposalID); err != nil {
		return summary, err
	}
	if err := db.RebuildCurriculumProjection(tx, proposalID); err != nil {
		return summary, err
	}
	if err := tx.Commit(); err != nil {
		return summary, fmt.Errorf("commit curriculum proposal publication: %w", err)
	}
	return RebaseDraftCurriculumProposals(database), nil
}

func sameOptionalID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// PopulateCurriculumProposalPreviousState derives display-only historical
// values by replaying a proposal over its frozen base. These values are never
// persisted in proposal change payloads.
func PopulateCurriculumProposalPreviousState(
	base *models.CurriculumGraph,
	proposal *models.CurriculumProposal,
) {
	if base == nil || proposal == nil {
		return
	}
	units := make(map[int64]models.Unit, len(base.Units))
	for _, unit := range base.Units {
		units[unit.ID] = unit
	}
	for _, index := range canonicalCurriculumProposalChangeIndexes(proposal.Changes) {
		change := &proposal.Changes[index]
		unit, exists := units[change.UnitID]
		switch change.Kind {
		case "create_unit":
			units[change.UnitID] = models.Unit{
				ID: change.UnitID, Name: change.UnitName, Content: change.UnitContent,
			}
		case "rename_unit":
			if exists {
				change.PreviousUnitName = unit.Name
				unit.Name = change.UnitName
				units[change.UnitID] = unit
			}
		case "update_content":
			if exists {
				change.PreviousUnitContent = unit.Content
				unit.Content = change.UnitContent
				units[change.UnitID] = unit
			}
		case "delete_unit":
			if exists {
				change.UnitName = unit.Name
				change.UnitContent = unit.Content
				delete(units, change.UnitID)
			}
		}
	}
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

func replaceUnitProposalChange(database *sql.DB, authorID, proposalID, unitID int64, kind string, change *models.CurriculumProposalChange) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := currentDraftCurriculumProposal(tx, authorID, proposalID); err != nil {
		return err
	}
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

func currentDraftCurriculumProposal(
	tx *sql.Tx,
	authorID, proposalID int64,
) (*models.CurriculumProposal, error) {
	proposal, err := db.GetCurriculumProposal(tx, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.Status != "draft" || !proposal.HasAuthor(authorID) {
		return nil, ErrProposalNotFound
	}
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return nil, err
	}
	if !sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return nil, ErrProposalRebaseRequired
	}
	return proposal, nil
}
