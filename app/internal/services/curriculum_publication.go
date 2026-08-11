package services

import (
	"database/sql"
	"fmt"

	"universal-curriculum/internal/db"
)

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
	if err := db.MigrateLearningPathTargets(tx, proposalID); err != nil {
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
