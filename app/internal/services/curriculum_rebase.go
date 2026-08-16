package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

const (
	ProposalRebaseCurrent     = "current"
	ProposalRebaseAutomatic   = "automatic"
	ProposalRebaseNeedsReview = "needs_review"
)

var (
	ErrProposalRebaseRequired   = errors.New("curriculum proposal requires rebase review")
	ErrRebaseResolutionRequired = errors.New("every rebase conflict requires a resolution")
)

type CurriculumProposalRebasePlan struct {
	Status            string
	CurrentProposalID *int64
	BaseProposal      *models.CurriculumProposal
	AcceptedProposals []models.CurriculumProposal
	Conflicts         []CurriculumProposalRebaseConflict
	ValidationReason  string
}

func (plan *CurriculumProposalRebasePlan) NeedsReview() bool {
	return plan != nil && plan.Status == ProposalRebaseNeedsReview
}

type CurriculumProposalRebaseConflict struct {
	Change       models.CurriculumProposalChange
	AcceptedUnit *models.Unit
	Units        []models.Unit
	AcceptedWork []CurriculumProposalRebaseAcceptedWork
}

type CurriculumProposalRebaseResolution struct {
	Choice  string
	Content string
}

type CurriculumProposalRebaseAcceptedWork struct {
	Proposal models.CurriculumProposal
	Changes  []models.CurriculumProposalChange
}

type CurriculumProposalRebaseSummary struct {
	AutomaticallyRebased int
	NeedsReview          int
	Failures             error
}

func CurriculumGraphAtProposal(
	database *sql.DB,
	proposalID *int64,
) (*models.CurriculumGraph, error) {
	graph := &models.CurriculumGraph{}
	if proposalID == nil {
		return graph, nil
	}
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin historical curriculum read: %w", err)
	}
	defer tx.Rollback()
	proposals, err := acceptedCurriculumProposalsSince(context.Background(), tx, nil, proposalID)
	if err != nil {
		return nil, err
	}
	for index := range proposals {
		graph = CurriculumGraphWithProposal(graph, &proposals[index])
	}
	names := make(map[int64]string, len(graph.Units))
	for _, unit := range graph.Units {
		names[unit.ID] = unit.Name
	}
	for index := range graph.Dependencies {
		graph.Dependencies[index].UnitName = names[graph.Dependencies[index].UnitID]
		graph.Dependencies[index].PrerequisiteName = names[graph.Dependencies[index].PrerequisiteID]
	}
	return graph, nil
}

func PlanCurriculumProposalRebase(
	ctx context.Context,
	database *sql.DB,
	proposal *models.CurriculumProposal,
) (*CurriculumProposalRebasePlan, error) {
	if proposal == nil {
		return nil, ErrProposalNotFound
	}
	tx, err := database.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := db.WithContext(ctx, tx)
	storedProposal, err := db.GetCurriculumProposal(q, proposal.ID)
	if err != nil {
		return nil, err
	}
	if storedProposal == nil || storedProposal.Status != "draft" {
		return nil, ErrProposalNotFound
	}
	proposal = storedProposal
	currentProposalID, err := db.GetCurrentCurriculumProposalID(q)
	if err != nil {
		return nil, err
	}
	if sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return currentCurriculumProposalRebasePlan(currentProposalID), nil
	}
	graph, err := db.GetCurriculumGraphWithContent(q)
	if err != nil {
		return nil, err
	}
	return planCurriculumProposalRebase(ctx, tx, proposal, currentProposalID, graph)
}

func TryAutoRebaseCurriculumProposal(
	database *sql.DB,
	proposalID int64,
) (*CurriculumProposalRebasePlan, error) {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	proposal, currentProposalID, err := loadProposalRebaseState(tx, proposalID)
	if err != nil {
		return nil, err
	}
	if sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return currentCurriculumProposalRebasePlan(currentProposalID), nil
	}
	graph, err := db.GetCurriculumGraphWithContent(tx)
	if err != nil {
		return nil, err
	}
	plan, err := planCurriculumProposalRebase(context.Background(), tx, proposal, currentProposalID, graph)
	if err != nil {
		return nil, err
	}
	if plan.Status != ProposalRebaseAutomatic {
		return plan, nil
	}
	updated, err := db.SetDraftCurriculumProposalBase(
		tx, proposal.ID, proposal.BaseProposalID, currentProposalID,
	)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, ErrProposalOutdated
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit automatic curriculum proposal rebase: %w", err)
	}
	plan.Status = ProposalRebaseCurrent
	return plan, nil
}

func RebaseDraftCurriculumProposals(database *sql.DB) CurriculumProposalRebaseSummary {
	ids, err := db.ListDraftCurriculumProposalIDs(database)
	if err != nil {
		return CurriculumProposalRebaseSummary{Failures: err}
	}
	var summary CurriculumProposalRebaseSummary
	var failures []error
	for _, proposalID := range ids {
		plan, err := TryAutoRebaseCurriculumProposal(database, proposalID)
		if err != nil {
			failures = append(failures, fmt.Errorf("rebase draft proposal %d: %w", proposalID, err))
			continue
		}
		switch plan.Status {
		case ProposalRebaseCurrent:
			if len(plan.AcceptedProposals) > 0 {
				summary.AutomaticallyRebased++
			}
		case ProposalRebaseNeedsReview:
			summary.NeedsReview++
		}
	}
	summary.Failures = errors.Join(failures...)
	return summary
}

func ResolveCurriculumProposalRebase(
	database *sql.DB,
	authorID, proposalID int64,
	resolutions map[int64]CurriculumProposalRebaseResolution,
) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, currentProposalID, err := loadProposalRebaseState(tx, proposalID)
	if err != nil {
		return err
	}
	if !proposal.HasAuthor(authorID) {
		return ErrProposalNotFound
	}
	if sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return nil
	}
	graph, err := db.GetCurriculumGraphWithContent(tx)
	if err != nil {
		return err
	}
	plan, err := planCurriculumProposalRebase(context.Background(), tx, proposal, currentProposalID, graph)
	if err != nil {
		return err
	}
	if plan.Status == ProposalRebaseCurrent {
		return nil
	}
	if plan.Status == ProposalRebaseAutomatic {
		updated, err := db.SetDraftCurriculumProposalBase(
			tx, proposal.ID, proposal.BaseProposalID, currentProposalID,
		)
		if err != nil {
			return err
		}
		if !updated {
			return ErrProposalOutdated
		}
		return commitCurriculumProposalRebase(tx)
	}

	conflictingChanges := make(map[int64]bool, len(plan.Conflicts))
	for _, conflict := range plan.Conflicts {
		conflictingChanges[conflict.Change.ID] = true
		resolution := resolutions[conflict.Change.ID]
		if resolution.Choice != "keep" && resolution.Choice != "drop" &&
			(resolution.Choice != "edit" || conflict.Change.Kind != "update_content" || strings.TrimSpace(resolution.Content) == "") {
			return ErrRebaseResolutionRequired
		}
	}
	candidate := *proposal
	candidate.Changes = append([]models.CurriculumProposalChange(nil), proposal.Changes...)
	retained := candidate.Changes[:0]
	for _, change := range candidate.Changes {
		if conflictingChanges[change.ID] {
			resolution := resolutions[change.ID]
			if resolution.Choice == "drop" {
				continue
			}
			if resolution.Choice == "edit" {
				change.UnitContent = strings.TrimSpace(resolution.Content)
			}
		}
		retained = append(retained, change)
	}
	candidate.Changes = retained
	normalizeProposalForRebase(&candidate, graph)
	if len(candidate.Changes) > 0 {
		if err := validateCurriculumProposal(graph, &candidate); err != nil {
			return err
		}
	}

	retainedIDs := make(map[int64]bool, len(candidate.Changes))
	for _, change := range candidate.Changes {
		retainedIDs[change.ID] = true
	}
	for _, change := range proposal.Changes {
		if retainedIDs[change.ID] {
			continue
		}
		deleted, err := db.DeleteDraftCurriculumProposalChange(tx, proposal.ID, change.ID, authorID)
		if err != nil {
			return err
		}
		if !deleted {
			return ErrProposalNotFound
		}
	}
	for _, change := range candidate.Changes {
		if err := db.UpdateDraftCurriculumProposalChangeForRebase(tx, change); err != nil {
			return err
		}
	}
	updated, err := db.SetDraftCurriculumProposalBase(
		tx, proposal.ID, proposal.BaseProposalID, currentProposalID,
	)
	if err != nil {
		return err
	}
	if !updated {
		return ErrProposalOutdated
	}
	return commitCurriculumProposalRebase(tx)
}

func loadProposalRebaseState(
	tx *sql.Tx,
	proposalID int64,
) (*models.CurriculumProposal, *int64, error) {
	currentProposalID, err := db.LockCurrentCurriculumProposalShared(tx)
	if err != nil {
		return nil, nil, err
	}
	locked, err := db.LockCurriculumProposal(tx, proposalID)
	if err != nil {
		return nil, nil, err
	}
	if !locked {
		return nil, nil, ErrProposalNotFound
	}
	proposal, err := db.GetCurriculumProposal(tx, proposalID)
	if err != nil {
		return nil, nil, err
	}
	if proposal == nil || proposal.Status != "draft" {
		return nil, nil, ErrProposalNotFound
	}
	return proposal, currentProposalID, nil
}

func currentCurriculumProposalRebasePlan(currentProposalID *int64) *CurriculumProposalRebasePlan {
	return &CurriculumProposalRebasePlan{
		Status: ProposalRebaseCurrent, CurrentProposalID: currentProposalID,
	}
}

func planCurriculumProposalRebase(
	ctx context.Context,
	q *sql.Tx,
	proposal *models.CurriculumProposal,
	currentProposalID *int64,
	currentGraph *models.CurriculumGraph,
) (*CurriculumProposalRebasePlan, error) {
	plan := currentCurriculumProposalRebasePlan(currentProposalID)
	if sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return plan, nil
	}
	accepted, err := acceptedCurriculumProposalsSince(ctx, q, proposal.BaseProposalID, currentProposalID)
	if err != nil {
		return nil, err
	}
	plan.AcceptedProposals = accepted
	if proposal.BaseProposalID != nil {
		plan.BaseProposal, err = db.GetCurriculumProposal(db.WithContext(ctx, q), *proposal.BaseProposalID)
		if err != nil {
			return nil, err
		}
	}
	upstreamTouches := make(map[int64][]int)
	for proposalIndex, acceptedProposal := range accepted {
		for _, change := range acceptedProposal.Changes {
			for _, unitID := range curriculumChangeUnitIDs(change) {
				upstreamTouches[unitID] = appendUniqueInt(upstreamTouches[unitID], proposalIndex)
			}
		}
	}
	names := curriculumRebaseUnitNames(currentGraph, proposal, accepted)
	for _, change := range proposal.Changes {
		proposalIndexes := make(map[int]bool)
		conflictingUnitIDs := make(map[int64]bool)
		var units []models.Unit
		for _, unitID := range curriculumChangeUnitIDs(change) {
			indexes := upstreamTouches[unitID]
			if len(indexes) == 0 {
				continue
			}
			units = append(units, models.Unit{ID: unitID, Name: names[unitID]})
			conflictingUnitIDs[unitID] = true
			for _, proposalIndex := range indexes {
				proposalIndexes[proposalIndex] = true
			}
		}
		if len(units) == 0 {
			continue
		}
		sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
		conflict := CurriculumProposalRebaseConflict{Change: change, Units: units}
		if change.Kind == "update_content" || change.Kind == "rename_unit" {
			if unit := currentGraph.Unit(change.UnitID); unit != nil {
				acceptedUnit := *unit
				conflict.AcceptedUnit = &acceptedUnit
			}
		}
		for proposalIndex, acceptedProposal := range accepted {
			if !proposalIndexes[proposalIndex] {
				continue
			}
			work := CurriculumProposalRebaseAcceptedWork{Proposal: acceptedProposal}
			for _, acceptedChange := range acceptedProposal.Changes {
				if !curriculumChangeTouchesAnyUnit(acceptedChange, conflictingUnitIDs) {
					continue
				}
				if acceptedChange.UnitName == "" {
					acceptedChange.UnitName = names[acceptedChange.UnitID]
				}
				work.Changes = append(work.Changes, acceptedChange)
			}
			conflict.AcceptedWork = append(conflict.AcceptedWork, work)
		}
		plan.Conflicts = append(plan.Conflicts, conflict)
	}
	if len(plan.Conflicts) == 0 {
		if len(proposal.Changes) == 0 {
			plan.Status = ProposalRebaseAutomatic
			return plan, nil
		}
		if err := validateCurriculumProposal(currentGraph, proposal); err == nil {
			plan.Status = ProposalRebaseAutomatic
			return plan, nil
		} else {
			plan.ValidationReason = err.Error()
			for _, change := range proposal.Changes {
				conflict := CurriculumProposalRebaseConflict{Change: change}
				if change.Kind == "update_content" || change.Kind == "rename_unit" {
					if unit := currentGraph.Unit(change.UnitID); unit != nil {
						acceptedUnit := *unit
						conflict.AcceptedUnit = &acceptedUnit
					}
				}
				plan.Conflicts = append(plan.Conflicts, conflict)
			}
		}
	}
	plan.Status = ProposalRebaseNeedsReview
	return plan, nil
}

func acceptedCurriculumProposalsSince(
	ctx context.Context,
	q *sql.Tx,
	baseProposalID, currentProposalID *int64,
) ([]models.CurriculumProposal, error) {
	if currentProposalID == nil {
		if baseProposalID == nil {
			return nil, nil
		}
		return nil, ErrProposalOutdated
	}
	cursor := currentProposalID
	visited := make(map[int64]bool)
	var reverse []models.CurriculumProposal
	for !sameOptionalID(cursor, baseProposalID) {
		if cursor == nil || visited[*cursor] {
			return nil, ErrProposalOutdated
		}
		visited[*cursor] = true
		proposal, err := db.GetCurriculumProposal(db.WithContext(ctx, q), *cursor)
		if err != nil {
			return nil, err
		}
		if proposal == nil || proposal.Status != "accepted" {
			return nil, ErrProposalOutdated
		}
		reverse = append(reverse, *proposal)
		cursor = proposal.BaseProposalID
	}
	slices.Reverse(reverse)
	return reverse, nil
}

func curriculumChangeUnitIDs(change models.CurriculumProposalChange) []int64 {
	ids := make([]int64, 0, 4)
	appendID := func(id int64) {
		if id > 0 && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if change.Kind != "recognition" {
		appendID(change.UnitID)
	}
	if change.PrerequisiteID != nil {
		appendID(*change.PrerequisiteID)
	}
	if change.Recognition != nil {
		for _, unit := range change.Recognition.Sources {
			appendID(unit.ID)
		}
		for _, unit := range change.Recognition.Targets {
			appendID(unit.ID)
		}
	}
	return ids
}

func curriculumChangeTouchesAnyUnit(
	change models.CurriculumProposalChange,
	unitIDs map[int64]bool,
) bool {
	for _, unitID := range curriculumChangeUnitIDs(change) {
		if unitIDs[unitID] {
			return true
		}
	}
	return false
}

func normalizeProposalForRebase(proposal *models.CurriculumProposal, current *models.CurriculumGraph) {
	created := make(map[int64]bool)
	for _, change := range proposal.Changes {
		if change.Kind == "create_unit" {
			created[change.UnitID] = true
		}
	}
	retained := proposal.Changes[:0]
	for _, change := range proposal.Changes {
		unit := current.Unit(change.UnitID)
		switch change.Kind {
		case "rename_unit":
			if !created[change.UnitID] && unit != nil {
				if change.UnitName == unit.Name {
					continue
				}
			}
		case "update_content":
			if !created[change.UnitID] && unit != nil {
				if change.UnitContent == unit.Content {
					continue
				}
			}
		case "add_dependency":
			if change.PrerequisiteID != nil && current.HasDependency(change.UnitID, *change.PrerequisiteID) {
				continue
			}
		case "remove_dependency":
			if change.PrerequisiteID != nil && !current.HasDependency(change.UnitID, *change.PrerequisiteID) {
				continue
			}
		}
		retained = append(retained, change)
	}
	proposal.Changes = retained
}

func curriculumRebaseUnitNames(
	current *models.CurriculumGraph,
	proposal *models.CurriculumProposal,
	accepted []models.CurriculumProposal,
) map[int64]string {
	names := make(map[int64]string)
	if current != nil {
		for _, unit := range current.Units {
			names[unit.ID] = unit.Name
		}
	}
	collect := func(change models.CurriculumProposalChange) {
		if change.UnitName != "" {
			names[change.UnitID] = change.UnitName
		}
		if change.Recognition != nil {
			for _, unit := range append(change.Recognition.Sources, change.Recognition.Targets...) {
				if unit.Name != "" {
					names[unit.ID] = unit.Name
				}
			}
		}
	}
	for _, acceptedProposal := range accepted {
		for _, change := range acceptedProposal.Changes {
			collect(change)
		}
	}
	for _, change := range proposal.Changes {
		collect(change)
	}
	return names
}

func appendUniqueInt(values []int, value int) []int {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func commitCurriculumProposalRebase(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum proposal rebase: %w", err)
	}
	return nil
}
