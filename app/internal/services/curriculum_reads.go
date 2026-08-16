package services

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

const (
	CurriculumUnitSearchDependency        = "dependency"
	CurriculumUnitSearchRecognitionSource = "recognition-source"
	CurriculumUnitSearchRecognitionTarget = "recognition-target"
)

// CurriculumProposalUnit resolves one unit in a proposal's resulting state.
// Previous is the unit in the proposal's frozen base and Historical is true
// when the proposal deletes the unit and only that base version remains visible.
func CurriculumProposalUnit(
	ctx context.Context,
	database *sql.DB,
	proposal *models.CurriculumProposal,
	unitID int64,
) (unit, previous *models.Unit, historical bool, err error) {
	if proposal == nil {
		unit, err = db.GetUnit(db.WithContext(ctx, database), unitID)
		return unit, nil, false, err
	}
	if proposal.BaseProposalID != nil {
		previous, err = db.GetCurriculumUnitAtProposal(ctx, database, *proposal.BaseProposalID, unitID)
		if err != nil {
			return nil, nil, false, err
		}
	}
	unit = cloneCurriculumUnit(previous)
	for _, index := range canonicalCurriculumProposalChangeIndexes(proposal.Changes) {
		change := proposal.Changes[index]
		if change.UnitID != unitID {
			continue
		}
		switch change.Kind {
		case "create_unit":
			unit = &models.Unit{ID: unitID, Name: change.UnitName, Content: change.UnitContent}
		case "rename_unit":
			if unit != nil {
				unit.Name = change.UnitName
			}
		case "update_content":
			if unit != nil {
				unit.Content = change.UnitContent
			}
		case "delete_unit":
			unit = nil
		}
	}
	if unit == nil && previous != nil {
		return cloneCurriculumUnit(previous), previous, true, nil
	}
	return unit, previous, false, nil
}

func CurriculumProposalUnitDependencies(
	ctx context.Context,
	database *sql.DB,
	proposal *models.CurriculumProposal,
	unitID int64,
) ([]models.UnitDependency, error) {
	var dependencies []models.UnitDependency
	var err error
	if proposal.BaseProposalID != nil {
		dependencies, err = db.GetCurriculumUnitDependenciesAtProposal(
			ctx, database, *proposal.BaseProposalID, unitID,
		)
		if err != nil {
			return nil, err
		}
	}
	edges := make(map[[2]int64]bool, len(dependencies))
	for _, dependency := range dependencies {
		edges[[2]int64{dependency.UnitID, dependency.PrerequisiteID}] = true
	}
	for _, change := range proposal.Changes {
		switch change.Kind {
		case "add_dependency", "remove_dependency":
			if change.UnitID != unitID && (change.PrerequisiteID == nil || *change.PrerequisiteID != unitID) {
				continue
			}
			edge := [2]int64{change.UnitID, *change.PrerequisiteID}
			if change.Kind == "add_dependency" {
				edges[edge] = true
			} else {
				delete(edges, edge)
			}
		case "delete_unit":
			for edge := range edges {
				if edge[0] == change.UnitID || edge[1] == change.UnitID {
					delete(edges, edge)
				}
			}
		}
	}
	dependencies = dependencies[:0]
	for edge := range edges {
		dependencies = append(dependencies, models.UnitDependency{UnitID: edge[0], PrerequisiteID: edge[1]})
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].UnitID == dependencies[j].UnitID {
			return dependencies[i].PrerequisiteID < dependencies[j].PrerequisiteID
		}
		return dependencies[i].UnitID < dependencies[j].UnitID
	})
	return dependencies, nil
}

func SearchCurriculumProposalUnits(
	ctx context.Context,
	database *sql.DB,
	proposal *models.CurriculumProposal,
	scope, query string,
	limit int,
) ([]models.Unit, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil, nil
	}
	if scope != CurriculumUnitSearchDependency && scope != CurriculumUnitSearchRecognitionSource && scope != CurriculumUnitSearchRecognitionTarget {
		return nil, ErrCurriculumUnitSearchScope
	}
	currentProposalID, err := db.GetCurrentCurriculumProposalID(db.WithContext(ctx, database))
	if err != nil {
		return nil, err
	}
	if proposal == nil || sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		if proposal != nil && scope != CurriculumUnitSearchRecognitionSource {
			return db.SearchDraftCurriculumUnitNames(ctx, database, proposal.ID, query, limit)
		}
		return db.SearchCurriculumUnitNames(ctx, database, query, limit)
	}
	base, err := curriculumProposalBaseGraph(ctx, database, proposal)
	if err != nil {
		return nil, err
	}
	var graph *models.CurriculumGraph
	switch scope {
	case CurriculumUnitSearchRecognitionSource:
		graph = base
	case CurriculumUnitSearchDependency, CurriculumUnitSearchRecognitionTarget:
		graph = CurriculumGraphWithProposal(base, proposal)
	}
	normalized := strings.ToLower(query)
	results := make([]models.Unit, 0, min(limit, len(graph.Units)))
	for _, unit := range graph.Units {
		if strings.Contains(strings.ToLower(unit.Name), normalized) {
			results = append(results, models.Unit{ID: unit.ID, Name: unit.Name})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		left, right := strings.ToLower(results[i].Name), strings.ToLower(results[j].Name)
		if left == right {
			return results[i].ID < results[j].ID
		}
		return left < right
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func curriculumProposalBaseGraph(
	ctx context.Context,
	database *sql.DB,
	proposal *models.CurriculumProposal,
) (*models.CurriculumGraph, error) {
	if proposal == nil {
		return db.GetCurriculumGraph(db.WithContext(ctx, database))
	}
	currentProposalID, err := db.GetCurrentCurriculumProposalID(db.WithContext(ctx, database))
	if err != nil {
		return nil, err
	}
	if sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return db.GetCurriculumGraph(db.WithContext(ctx, database))
	}
	return CurriculumGraphAtProposal(ctx, database, proposal.BaseProposalID)
}

func cloneCurriculumUnit(unit *models.Unit) *models.Unit {
	if unit == nil {
		return nil
	}
	copy := *unit
	return &copy
}
