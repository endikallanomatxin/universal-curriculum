package server

import (
	"time"

	"universal-curriculum/internal/models"
)

type apiProposal struct {
	ID             int64               `json:"id"`
	Title          string              `json:"title"`
	Rationale      string              `json:"rationale"`
	Status         string              `json:"status"`
	BaseProposalID *int64              `json:"base_proposal_id"`
	AuthorIDs      []int64             `json:"author_ids"`
	AuthorName     string              `json:"author_name"`
	ChangeCount    int                 `json:"change_count"`
	CreatedAt      time.Time           `json:"created_at"`
	AcceptedAt     *time.Time          `json:"accepted_at"`
	Changes        []apiProposalChange `json:"changes,omitempty"`
}

type apiProposalChange struct {
	ID             int64           `json:"id"`
	Kind           string          `json:"kind"`
	UnitID         *int64          `json:"unit_id,omitempty"`
	UnitName       string          `json:"unit_name,omitempty"`
	UnitContent    string          `json:"unit_content,omitempty"`
	PrerequisiteID *int64          `json:"prerequisite_id,omitempty"`
	Recognition    *apiRecognition `json:"recognition,omitempty"`
}

type apiRecognition struct {
	SourceUnitIDs []int64 `json:"source_unit_ids"`
	TargetUnitIDs []int64 `json:"target_unit_ids"`
}

func newAPIProposal(proposal models.CurriculumProposal, includeChanges bool) apiProposal {
	resource := apiProposal{
		ID: proposal.ID, Title: proposal.Title, Rationale: proposal.Rationale,
		Status: proposal.Status, BaseProposalID: proposal.BaseProposalID,
		AuthorIDs: append([]int64{}, proposal.AuthorIDs...), AuthorName: proposal.AuthorName,
		ChangeCount: proposal.ChangeCount, CreatedAt: proposal.CreatedAt, AcceptedAt: proposal.AcceptedAt,
	}
	if !includeChanges {
		return resource
	}
	resource.Changes = make([]apiProposalChange, 0, len(proposal.Changes))
	resource.ChangeCount = len(proposal.Changes)
	for _, change := range proposal.Changes {
		item := apiProposalChange{
			ID: change.ID, Kind: change.Kind, UnitName: change.UnitName,
			UnitContent: change.UnitContent, PrerequisiteID: change.PrerequisiteID,
		}
		if change.UnitID > 0 {
			unitID := change.UnitID
			item.UnitID = &unitID
		}
		if change.Recognition != nil {
			item.Recognition = &apiRecognition{SourceUnitIDs: []int64{}, TargetUnitIDs: []int64{}}
			for _, unit := range change.Recognition.Sources {
				item.Recognition.SourceUnitIDs = append(item.Recognition.SourceUnitIDs, unit.ID)
			}
			for _, unit := range change.Recognition.Targets {
				item.Recognition.TargetUnitIDs = append(item.Recognition.TargetUnitIDs, unit.ID)
			}
		}
		resource.Changes = append(resource.Changes, item)
	}
	return resource
}
