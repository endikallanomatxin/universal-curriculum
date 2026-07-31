package services

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"universal-curriculum/internal/models"
)

var ErrProposalInvalid = errors.New("curriculum proposal is invalid")

type ProposalValidationError struct {
	ChangeID int64
	Reason   string
}

func (err *ProposalValidationError) Error() string {
	if err.ChangeID == 0 {
		return "proposal is invalid: " + err.Reason
	}
	return fmt.Sprintf("proposal change %d is invalid: %s", err.ChangeID, err.Reason)
}

func (err *ProposalValidationError) Unwrap() error {
	return ErrProposalInvalid
}

type curriculumDependencyKey struct {
	unitID         int64
	prerequisiteID int64
}

type curriculumProposalValidationState struct {
	units               map[int64]models.Unit
	dependencies        map[curriculumDependencyKey]struct{}
	createdUnits        map[int64]bool
	renamedUnits        map[int64]bool
	contentUpdatedUnits map[int64]bool
	deletedUnits        map[int64]bool
	changedDependencies map[curriculumDependencyKey]bool
	baseUnits           map[int64]bool
	knowledgeTransfers  map[string]bool
}

func validateCurriculumProposal(base *models.CurriculumGraph, proposal *models.CurriculumProposal) error {
	if base == nil {
		return invalidProposal("the base curriculum is unavailable")
	}
	if proposal == nil || len(proposal.Changes) == 0 {
		return ErrProposalEmpty
	}
	state := &curriculumProposalValidationState{
		units:               make(map[int64]models.Unit, len(base.Units)),
		dependencies:        make(map[curriculumDependencyKey]struct{}, len(base.Dependencies)),
		createdUnits:        make(map[int64]bool),
		renamedUnits:        make(map[int64]bool),
		contentUpdatedUnits: make(map[int64]bool),
		deletedUnits:        make(map[int64]bool),
		changedDependencies: make(map[curriculumDependencyKey]bool),
		baseUnits:           make(map[int64]bool, len(base.Units)),
		knowledgeTransfers:  make(map[string]bool),
	}
	for _, unit := range base.Units {
		if unit.ID <= 0 || !validCurriculumText(unit.Name) || !validCurriculumText(unit.Content) {
			return invalidProposal("the base curriculum contains an invalid unit")
		}
		if _, exists := state.units[unit.ID]; exists {
			return invalidProposal("the base curriculum contains duplicate unit identities")
		}
		state.units[unit.ID] = unit
		state.baseUnits[unit.ID] = true
	}
	for _, dependency := range base.Dependencies {
		key := curriculumDependencyKey{dependency.UnitID, dependency.PrerequisiteID}
		if dependency.UnitID == dependency.PrerequisiteID {
			return invalidProposal("the base curriculum contains a self-dependency")
		}
		if _, exists := state.units[dependency.UnitID]; !exists {
			return invalidProposal("the base curriculum contains a dependency with a missing unit")
		}
		if _, exists := state.units[dependency.PrerequisiteID]; !exists {
			return invalidProposal("the base curriculum contains a dependency with a missing prerequisite")
		}
		if _, exists := state.dependencies[key]; exists {
			return invalidProposal("the base curriculum contains a duplicate dependency")
		}
		if state.dependencyCreatesCycle(key) {
			return invalidProposal("the base curriculum contains a dependency cycle")
		}
		state.dependencies[key] = struct{}{}
	}

	lastPosition := 0
	var knowledgeTransfers []models.CurriculumProposalChange
	for _, change := range proposal.Changes {
		if change.ID <= 0 || change.Position <= lastPosition {
			return invalidChange(change, "changes are not uniquely ordered")
		}
		lastPosition = change.Position
		if change.Kind == "transfer_knowledge" {
			knowledgeTransfers = append(knowledgeTransfers, change)
			continue
		}
		if err := state.apply(change); err != nil {
			return err
		}
	}
	for _, change := range knowledgeTransfers {
		if err := state.validateKnowledgeTransfer(change); err != nil {
			return err
		}
	}
	return nil
}

func (state *curriculumProposalValidationState) apply(change models.CurriculumProposalChange) error {
	switch change.Kind {
	case "create_unit":
		return state.createUnit(change)
	case "rename_unit":
		return state.renameUnit(change)
	case "update_content":
		return state.updateUnitContent(change)
	case "delete_unit":
		return state.deleteUnit(change)
	case "add_dependency":
		return state.addDependency(change)
	case "remove_dependency":
		return state.removeDependency(change)
	default:
		return invalidChange(change, fmt.Sprintf("unsupported change kind %q", change.Kind))
	}
}

func (state *curriculumProposalValidationState) validateKnowledgeTransfer(
	change models.CurriculumProposalChange,
) error {
	transfer := change.KnowledgeTransfer
	if transfer == nil {
		return invalidChange(change, "the knowledge transfer detail is missing")
	}
	if !validCurriculumText(transfer.Rationale) {
		return invalidChange(change, "the knowledge transfer rationale is empty or not normalized")
	}
	if len(transfer.Sources) == 0 {
		return invalidChange(change, "the knowledge transfer has no source units")
	}
	if len(transfer.Targets) == 0 {
		return invalidChange(change, "the knowledge transfer has no target units")
	}
	sourceIDs := make([]int64, 0, len(transfer.Sources))
	seenSources := make(map[int64]bool, len(transfer.Sources))
	for _, source := range transfer.Sources {
		if source.ID <= 0 || seenSources[source.ID] {
			return invalidChange(change, "the knowledge transfer contains an invalid or duplicate source")
		}
		if !state.baseUnits[source.ID] {
			return invalidChange(change, "a knowledge transfer source is not present in the base curriculum")
		}
		seenSources[source.ID] = true
		sourceIDs = append(sourceIDs, source.ID)
	}
	targetIDs := make([]int64, 0, len(transfer.Targets))
	seenTargets := make(map[int64]bool, len(transfer.Targets))
	for _, target := range transfer.Targets {
		if target.ID <= 0 || seenTargets[target.ID] {
			return invalidChange(change, "the knowledge transfer contains an invalid or duplicate target")
		}
		if _, exists := state.units[target.ID]; !exists {
			return invalidChange(change, "a knowledge transfer target is not present in the resulting curriculum")
		}
		seenTargets[target.ID] = true
		targetIDs = append(targetIDs, target.ID)
	}
	sort.Slice(sourceIDs, func(i, j int) bool { return sourceIDs[i] < sourceIDs[j] })
	sort.Slice(targetIDs, func(i, j int) bool { return targetIDs[i] < targetIDs[j] })
	key := fmt.Sprintf("%v->%v", sourceIDs, targetIDs)
	if state.knowledgeTransfers[key] {
		return invalidChange(change, "the same knowledge transfer is declared more than once")
	}
	state.knowledgeTransfers[key] = true
	return nil
}

func (state *curriculumProposalValidationState) createUnit(change models.CurriculumProposalChange) error {
	if change.UnitID != change.ID {
		return invalidChange(change, "a unit creation must use the creation change as its identity")
	}
	if !validCurriculumText(change.UnitName) {
		return invalidChange(change, "the unit name is empty or not normalized")
	}
	if !validCurriculumText(change.UnitContent) {
		return invalidChange(change, "the unit content is empty or not normalized")
	}
	if _, exists := state.units[change.UnitID]; exists || state.createdUnits[change.UnitID] {
		return invalidChange(change, "the unit identity already exists")
	}
	state.units[change.UnitID] = models.Unit{
		ID: change.UnitID, Name: change.UnitName, Content: change.UnitContent,
	}
	state.createdUnits[change.UnitID] = true
	return nil
}

func (state *curriculumProposalValidationState) renameUnit(change models.CurriculumProposalChange) error {
	unit, exists := state.units[change.UnitID]
	if !exists {
		return invalidChange(change, "the unit to rename does not exist")
	}
	if state.renamedUnits[change.UnitID] {
		return invalidChange(change, "the same unit is renamed more than once")
	}
	if !validCurriculumText(change.UnitName) || !validCurriculumText(change.PreviousUnitName) {
		return invalidChange(change, "the unit names are empty or not normalized")
	}
	if change.PreviousUnitName != unit.Name {
		return invalidChange(change, "the previous name does not match the proposal state")
	}
	if change.UnitName == unit.Name {
		return invalidChange(change, "the rename has no effect")
	}
	unit.Name = change.UnitName
	state.units[change.UnitID] = unit
	state.renamedUnits[change.UnitID] = true
	return nil
}

func (state *curriculumProposalValidationState) updateUnitContent(change models.CurriculumProposalChange) error {
	unit, exists := state.units[change.UnitID]
	if !exists {
		return invalidChange(change, "the unit to update does not exist")
	}
	if state.contentUpdatedUnits[change.UnitID] {
		return invalidChange(change, "the same unit content is updated more than once")
	}
	if !validCurriculumText(change.UnitContent) || !validCurriculumText(change.PreviousUnitContent) {
		return invalidChange(change, "the unit content is empty or not normalized")
	}
	if change.PreviousUnitContent != unit.Content {
		return invalidChange(change, "the previous content does not match the proposal state")
	}
	if change.UnitContent == unit.Content {
		return invalidChange(change, "the content update has no effect")
	}
	unit.Content = change.UnitContent
	state.units[change.UnitID] = unit
	state.contentUpdatedUnits[change.UnitID] = true
	return nil
}

func (state *curriculumProposalValidationState) deleteUnit(change models.CurriculumProposalChange) error {
	if state.deletedUnits[change.UnitID] {
		return invalidChange(change, "the same unit is deleted more than once")
	}
	unit, exists := state.units[change.UnitID]
	if !exists {
		return invalidChange(change, "the unit to delete does not exist")
	}
	if state.createdUnits[change.UnitID] {
		return invalidChange(change, "a unit created and deleted by the same proposal must be omitted")
	}
	if state.renamedUnits[change.UnitID] || state.contentUpdatedUnits[change.UnitID] {
		return invalidChange(change, "changes superseded by the unit deletion must be omitted")
	}
	if change.UnitName != unit.Name || change.UnitContent != unit.Content {
		return invalidChange(change, "the deleted unit snapshot does not match the proposal state")
	}
	for dependency := range state.dependencies {
		if dependency.prerequisiteID == change.UnitID {
			return invalidChange(change, "another unit still depends on the unit being deleted")
		}
		if dependency.unitID == change.UnitID {
			if state.changedDependencies[dependency] {
				return invalidChange(change, "dependency changes superseded by the unit deletion must be omitted")
			}
			delete(state.dependencies, dependency)
		}
	}
	delete(state.units, change.UnitID)
	state.deletedUnits[change.UnitID] = true
	return nil
}

func (state *curriculumProposalValidationState) addDependency(change models.CurriculumProposalChange) error {
	key, err := state.validateDependencyChange(change)
	if err != nil {
		return err
	}
	if _, exists := state.dependencies[key]; exists {
		return invalidChange(change, "the dependency already exists")
	}
	if state.dependencyCreatesCycle(key) {
		return invalidChange(change, "the dependency creates a cycle")
	}
	state.dependencies[key] = struct{}{}
	state.changedDependencies[key] = true
	return nil
}

func (state *curriculumProposalValidationState) removeDependency(change models.CurriculumProposalChange) error {
	key, err := state.validateDependencyChange(change)
	if err != nil {
		return err
	}
	if _, exists := state.dependencies[key]; !exists {
		return invalidChange(change, "the dependency does not exist")
	}
	delete(state.dependencies, key)
	state.changedDependencies[key] = true
	return nil
}

func (state *curriculumProposalValidationState) validateDependencyChange(change models.CurriculumProposalChange) (curriculumDependencyKey, error) {
	if change.PrerequisiteID == nil {
		return curriculumDependencyKey{}, invalidChange(change, "the dependency has no prerequisite")
	}
	key := curriculumDependencyKey{change.UnitID, *change.PrerequisiteID}
	if key.unitID == key.prerequisiteID {
		return curriculumDependencyKey{}, invalidChange(change, "a unit cannot depend on itself")
	}
	if _, exists := state.units[key.unitID]; !exists {
		return curriculumDependencyKey{}, invalidChange(change, "the dependent unit does not exist")
	}
	if _, exists := state.units[key.prerequisiteID]; !exists {
		return curriculumDependencyKey{}, invalidChange(change, "the prerequisite unit does not exist")
	}
	if state.changedDependencies[key] {
		return curriculumDependencyKey{}, invalidChange(change, "the same dependency is changed more than once")
	}
	return key, nil
}

func (state *curriculumProposalValidationState) dependencyCreatesCycle(candidate curriculumDependencyKey) bool {
	dependents := make(map[int64][]int64, len(state.dependencies)+1)
	for dependency := range state.dependencies {
		dependents[dependency.prerequisiteID] = append(dependents[dependency.prerequisiteID], dependency.unitID)
	}
	dependents[candidate.prerequisiteID] = append(dependents[candidate.prerequisiteID], candidate.unitID)
	pending := []int64{candidate.unitID}
	visited := make(map[int64]bool)
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == candidate.prerequisiteID {
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

func validCurriculumText(value string) bool {
	return value != "" && value == strings.TrimSpace(value)
}

func invalidProposal(reason string) error {
	return &ProposalValidationError{Reason: reason}
}

func invalidChange(change models.CurriculumProposalChange, reason string) error {
	return &ProposalValidationError{ChangeID: change.ID, Reason: reason}
}
