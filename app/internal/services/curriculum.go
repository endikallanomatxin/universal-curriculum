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
	ErrUnitNameTooLong            = errors.New("unit name must not exceed 200 characters")
	ErrUnitContentRequired        = errors.New("unit content is required")
	ErrDependencyExists           = errors.New("unit dependency already exists")
	ErrDependencyNotFound         = errors.New("unit dependency not found")
	ErrDependencyCycle            = errors.New("unit dependency creates a cycle")
	ErrProposalNotFound           = errors.New("draft curriculum proposal not found")
	ErrProposalTitleRequired      = errors.New("proposal title is required")
	ErrProposalTitleTooLong       = errors.New("proposal title must not exceed 200 characters")
	ErrProposalRationaleRequired  = errors.New("proposal rationale is required")
	ErrProposalRationaleTooLong   = errors.New("proposal rationale must not exceed 1000 characters")
	ErrProposalEmpty              = errors.New("curriculum proposal has no changes")
	ErrProposalOutdated           = errors.New("curriculum proposal is not based on the current curriculum")
	ErrRecognitionSourcesRequired = errors.New("recognition requires at least one source")
	ErrRecognitionTargetsRequired = errors.New("recognition requires at least one target")
)

const (
	MaximumUnitNameLength          = 200
	MaximumProposalTitleLength     = 200
	MaximumProposalRationaleLength = 1000
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
	name, err := validateUnitName(name)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
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
	name, err := validateUnitName(name)
	if err != nil {
		return err
	}
	return updateCurriculumUnitFields(database, authorID, proposalID, unitID, &name, nil)
}

func UpdateCurriculumUnitContent(database *sql.DB, authorID, proposalID, unitID int64, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrUnitContentRequired
	}
	return updateCurriculumUnitFields(database, authorID, proposalID, unitID, nil, &content)
}

// UpdateCurriculumUnitAndContent replaces both editable fields in one
// transaction. It is used by representations that submit a complete unit
// rather than the web interface's independent inline fields.
func UpdateCurriculumUnitAndContent(
	database *sql.DB, authorID, proposalID, unitID int64, name, content string,
) error {
	name, err := validateUnitName(name)
	if err != nil {
		return err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrUnitContentRequired
	}
	return updateCurriculumUnitFields(database, authorID, proposalID, unitID, &name, &content)
}

func updateCurriculumUnitFields(
	database *sql.DB, authorID, proposalID, unitID int64, name, content *string,
) error {
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
	var creation *models.CurriculumProposalChange
	var deleted bool
	for _, change := range proposal.Changes {
		if change.Kind == "delete_unit" && change.UnitID == unitID {
			deleted = true
		}
		if change.Kind == "create_unit" && change.UnitID == unitID {
			copy := change
			creation = &copy
		}
	}
	if deleted {
		return ErrUnitNotFound
	}
	if creation != nil {
		finalName, finalContent := creation.UnitName, creation.UnitContent
		for _, change := range proposal.Changes {
			if change.UnitID != unitID {
				continue
			}
			switch change.Kind {
			case "rename_unit":
				finalName = change.UnitName
			case "update_content":
				finalContent = change.UnitContent
			}
		}
		if name != nil {
			finalName = *name
		}
		if content != nil {
			finalContent = *content
		}
		for _, change := range proposal.Changes {
			if change.UnitID != unitID || change.Kind != "rename_unit" && change.Kind != "update_content" {
				continue
			}
			deleted, err := db.DeleteDraftCurriculumProposalChange(tx, proposalID, change.ID, authorID)
			if err != nil {
				return err
			}
			if !deleted {
				return ErrProposalNotFound
			}
		}
		updated, err := db.UpdateDraftCurriculumUnitCreation(
			tx, proposalID, authorID, creation.ID, finalName, finalContent,
		)
		if err != nil {
			return err
		}
		if !updated {
			return ErrProposalNotFound
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit curriculum unit creation update: %w", err)
		}
		return nil
	}
	baseline, err := db.GetUnit(tx, unitID)
	if err != nil {
		return err
	}
	if baseline == nil {
		return ErrUnitNotFound
	}
	updates := []struct {
		kind   string
		change *models.CurriculumProposalChange
	}{
		{kind: "rename_unit"},
		{kind: "update_content"},
	}
	if name != nil && *name != baseline.Name {
		updates[0].change = &models.CurriculumProposalChange{
			Kind: "rename_unit", UnitID: unitID, UnitName: *name,
		}
	}
	if content != nil && *content != baseline.Content {
		updates[1].change = &models.CurriculumProposalChange{
			Kind: "update_content", UnitID: unitID, UnitContent: *content,
		}
	}
	for _, update := range updates {
		if (update.kind == "rename_unit" && name == nil) ||
			(update.kind == "update_content" && content == nil) {
			continue
		}
		authorized, err := db.DeleteDraftCurriculumProposalUnitChanges(
			tx, proposalID, authorID, unitID, update.kind,
		)
		if err != nil {
			return err
		}
		if !authorized {
			return ErrProposalNotFound
		}
		if update.change != nil {
			if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, update.change); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum unit replacement: %w", err)
	}
	return nil
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

// SetUnitDependency converges a dependency to the requested state. Repeating
// the same operation is a successful no-op.
func SetUnitDependency(
	database *sql.DB, authorID, proposalID, unitID, prerequisiteID int64, desired bool,
) error {
	if desired {
		err := AddUnitDependency(database, authorID, proposalID, unitID, prerequisiteID)
		if errors.Is(err, ErrDependencyExists) {
			return nil
		}
		return err
	}
	err := RemoveUnitDependency(database, authorID, proposalID, unitID, prerequisiteID)
	if errors.Is(err, ErrDependencyNotFound) {
		return nil
	}
	return err
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
	if len([]rune(title)) > MaximumProposalTitleLength {
		return "", "", ErrProposalTitleTooLong
	}
	if rationale == "" {
		return "", "", ErrProposalRationaleRequired
	}
	if len([]rune(rationale)) > MaximumProposalRationaleLength {
		return "", "", ErrProposalRationaleTooLong
	}
	return title, rationale, nil
}

func validateUnitName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrUnitNameRequired
	}
	if len([]rune(name)) > MaximumUnitNameLength {
		return "", ErrUnitNameTooLong
	}
	return name, nil
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
