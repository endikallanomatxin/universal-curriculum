package mcpadapter

import (
	"time"

	"universal-curriculum/internal/models"
)

type toolError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	Retryable bool              `json:"retryable"`
}

type toolOutput[T any] struct {
	OK    bool       `json:"ok"`
	Data  *T         `json:"data,omitempty"`
	Error *toolError `json:"error,omitempty"`
}

type unitSummary struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Retired bool   `json:"retired"`
}

type unit struct {
	unitSummary
	Content         string  `json:"content"`
	PrerequisiteIDs []int64 `json:"prerequisite_ids"`
	DependentIDs    []int64 `json:"dependent_ids"`
	CreatedAt       string  `json:"created_at,omitempty"`
	UpdatedAt       string  `json:"updated_at,omitempty"`
}

type dependency struct {
	UnitID         int64 `json:"unit_id"`
	PrerequisiteID int64 `json:"prerequisite_id"`
}

type curriculum struct {
	ProposalID   *int64       `json:"proposal_id,omitempty"`
	Units        []unit       `json:"units"`
	Dependencies []dependency `json:"dependencies"`
}

type curriculumOverview struct {
	ProposalID   *int64        `json:"proposal_id,omitempty"`
	Units        []unitSummary `json:"units"`
	Dependencies []dependency  `json:"dependencies"`
}

func newCurriculumOverview(value curriculum) curriculumOverview {
	result := curriculumOverview{
		ProposalID: value.ProposalID, Units: make([]unitSummary, 0, len(value.Units)),
		Dependencies: value.Dependencies,
	}
	for _, item := range value.Units {
		result.Units = append(result.Units, item.unitSummary)
	}
	return result
}

type learningPath struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	TargetUnits []unitSummary `json:"target_units"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

type progress struct {
	UnitID     int64 `json:"unit_id"`
	Direct     bool  `json:"direct"`
	Recognized bool  `json:"recognized"`
	Completed  bool  `json:"completed"`
}

type recommendation struct {
	LearningPathID   int64             `json:"learning_path_id"`
	LearningPathName string            `json:"learning_path_name"`
	Units            []recommendedUnit `json:"units"`
}

type recommendedUnit struct {
	unitSummary
	Reason string `json:"reason"`
}

type proposal struct {
	ID             int64            `json:"id"`
	Title          string           `json:"title"`
	Rationale      string           `json:"rationale"`
	Status         string           `json:"status"`
	BaseProposalID *int64           `json:"base_proposal_id,omitempty"`
	AuthorIDs      []int64          `json:"author_ids"`
	AuthorName     string           `json:"author_name"`
	ChangeCount    int              `json:"change_count"`
	CreatedAt      string           `json:"created_at"`
	AcceptedAt     *string          `json:"accepted_at,omitempty"`
	Changes        []proposalChange `json:"changes,omitempty"`
}

type proposalChange struct {
	ID             int64        `json:"id"`
	Kind           string       `json:"kind"`
	UnitID         *int64       `json:"unit_id,omitempty"`
	UnitName       string       `json:"unit_name,omitempty"`
	UnitContent    string       `json:"unit_content,omitempty"`
	PrerequisiteID *int64       `json:"prerequisite_id,omitempty"`
	Recognition    *recognition `json:"recognition,omitempty"`
}

type recognition struct {
	SourceUnitIDs []int64 `json:"source_unit_ids"`
	TargetUnitIDs []int64 `json:"target_unit_ids"`
}

type rebasePlan struct {
	Status              string           `json:"status"`
	AcceptedProposalIDs []int64          `json:"accepted_proposal_ids"`
	Conflicts           []rebaseConflict `json:"conflicts"`
}

type rebaseConflict struct {
	ChangeID            int64   `json:"change_id"`
	Kind                string  `json:"kind"`
	UnitID              *int64  `json:"unit_id,omitempty"`
	AcceptedProposalIDs []int64 `json:"accepted_proposal_ids"`
}

func newUnitSummary(model models.Unit) unitSummary {
	return unitSummary{ID: model.ID, Name: model.Name, Retired: model.Retired}
}

func curriculumRepresentation(graph *models.CurriculumGraph, proposalID *int64) curriculum {
	result := curriculum{ProposalID: proposalID, Units: []unit{}, Dependencies: []dependency{}}
	indexes := make(map[int64]int, len(graph.Units))
	for _, model := range graph.Units {
		indexes[model.ID] = len(result.Units)
		item := unit{
			unitSummary: newUnitSummary(model), Content: model.Content,
			PrerequisiteIDs: []int64{}, DependentIDs: []int64{},
		}
		if !model.CreatedAt.IsZero() {
			item.CreatedAt = model.CreatedAt.UTC().Format(time.RFC3339)
		}
		if !model.UpdatedAt.IsZero() {
			item.UpdatedAt = model.UpdatedAt.UTC().Format(time.RFC3339)
		}
		result.Units = append(result.Units, item)
	}
	for _, model := range graph.Dependencies {
		result.Dependencies = append(result.Dependencies, dependency{
			UnitID: model.UnitID, PrerequisiteID: model.PrerequisiteID,
		})
		if index, ok := indexes[model.UnitID]; ok {
			result.Units[index].PrerequisiteIDs = append(result.Units[index].PrerequisiteIDs, model.PrerequisiteID)
		}
		if index, ok := indexes[model.PrerequisiteID]; ok {
			result.Units[index].DependentIDs = append(result.Units[index].DependentIDs, model.UnitID)
		}
	}
	return result
}

func newLearningPath(model models.LearningPath) learningPath {
	result := learningPath{
		ID: model.ID, Name: model.Name, TargetUnits: make([]unitSummary, 0, len(model.Units)),
		CreatedAt: model.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: model.UpdatedAt.UTC().Format(time.RFC3339),
	}
	for _, target := range model.Units {
		result.TargetUnits = append(result.TargetUnits, newUnitSummary(target))
	}
	return result
}

func newProposal(model models.CurriculumProposal, includeChanges bool) proposal {
	result := proposal{
		ID: model.ID, Title: model.Title, Rationale: model.Rationale, Status: model.Status,
		BaseProposalID: model.BaseProposalID, AuthorIDs: append([]int64{}, model.AuthorIDs...),
		AuthorName: model.AuthorName, ChangeCount: model.ChangeCount,
		CreatedAt: model.CreatedAt.UTC().Format(time.RFC3339),
	}
	if model.AcceptedAt != nil {
		formatted := model.AcceptedAt.UTC().Format(time.RFC3339)
		result.AcceptedAt = &formatted
	}
	if !includeChanges {
		return result
	}
	result.Changes = make([]proposalChange, 0, len(model.Changes))
	result.ChangeCount = len(model.Changes)
	for _, change := range model.Changes {
		item := proposalChange{
			ID: change.ID, Kind: change.Kind, UnitName: change.UnitName,
			UnitContent: change.UnitContent, PrerequisiteID: change.PrerequisiteID,
		}
		if change.UnitID > 0 {
			id := change.UnitID
			item.UnitID = &id
		}
		if change.Recognition != nil {
			item.Recognition = &recognition{SourceUnitIDs: []int64{}, TargetUnitIDs: []int64{}}
			for _, source := range change.Recognition.Sources {
				item.Recognition.SourceUnitIDs = append(item.Recognition.SourceUnitIDs, source.ID)
			}
			for _, target := range change.Recognition.Targets {
				item.Recognition.TargetUnitIDs = append(item.Recognition.TargetUnitIDs, target.ID)
			}
		}
		result.Changes = append(result.Changes, item)
	}
	return result
}
