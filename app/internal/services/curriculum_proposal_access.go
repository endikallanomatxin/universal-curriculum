package services

import (
	"database/sql"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

// GetVisibleCurriculumProposal returns a proposal when it is public or the
// requesting user is one of its authors. Private drafts are indistinguishable
// from missing proposals to avoid exposing their existence.
func GetVisibleCurriculumProposal(database *sql.DB, userID, proposalID int64) (*models.CurriculumProposal, error) {
	proposal, err := db.GetCurriculumProposal(database, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.Status == "draft" && !proposal.HasAuthor(userID) {
		return nil, ErrProposalNotFound
	}
	return proposal, nil
}

// GetEditableCurriculumProposal returns an authored draft. Other proposal
// states and drafts owned by someone else follow the same not-found semantics
// as visibility checks.
func GetEditableCurriculumProposal(database *sql.DB, userID, proposalID int64) (*models.CurriculumProposal, error) {
	proposal, err := GetVisibleCurriculumProposal(database, userID, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Status != "draft" || !proposal.HasAuthor(userID) {
		return nil, ErrProposalNotFound
	}
	return proposal, nil
}
