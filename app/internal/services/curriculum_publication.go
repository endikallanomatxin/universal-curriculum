package services

import (
	"database/sql"
	"fmt"
	"strings"

	"universal-curriculum/internal/db"
)

func SubmitCurriculumProposal(
	database *sql.DB,
	authorID, proposalID int64,
) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proposal, err := db.GetCurriculumProposal(tx, proposalID)
	if err != nil {
		return err
	}
	if proposal == nil || proposal.Status != "draft" || !proposal.HasAuthor(authorID) {
		return ErrProposalNotFound
	}
	if len(proposal.Changes) == 0 {
		return ErrProposalEmpty
	}
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return err
	}
	graph, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return err
	}
	if !sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		plan, err := planCurriculumProposalRebase(tx, proposal, currentProposalID, graph)
		if err != nil {
			return err
		}
		if plan.Status != ProposalRebaseAutomatic {
			return ErrProposalRebaseRequired
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
		proposal.BaseProposalID = currentProposalID
	}
	if err := validateCurriculumProposal(graph, proposal); err != nil {
		return err
	}
	ok, err := db.SubmitDraftCurriculumProposal(tx, proposalID, authorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	return tx.Commit()
}

// PublishCurriculumProposal preserves the pre-0.3 workflow for internal
// callers while adapters migrate to the separate submission and decision
// operations.
func PublishCurriculumProposal(database *sql.DB, authorID, proposalID int64) (CurriculumProposalRebaseSummary, error) {
	if err := SubmitCurriculumProposal(database, authorID, proposalID); err != nil {
		return CurriculumProposalRebaseSummary{}, err
	}
	return AcceptCurriculumProposal(database, authorID, proposalID)
}

func AcceptCurriculumProposal(database *sql.DB, administratorID, proposalID int64) (CurriculumProposalRebaseSummary, error) {
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
	if proposal == nil || proposal.Status != "submitted" {
		return summary, ErrProposalNotFound
	}
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return summary, err
	}
	if !sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return summary, ErrProposalOutdated
	}
	graph, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return summary, err
	}
	if err := validateCurriculumProposal(graph, proposal); err != nil {
		return summary, err
	}
	ok, err := db.AcceptSubmittedCurriculumProposal(tx, proposalID, administratorID)
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

func RejectCurriculumProposal(database *sql.DB, administratorID, proposalID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ErrProposalRationaleRequired
	}
	ok, err := db.RejectSubmittedCurriculumProposal(database, proposalID, administratorID, reason)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	return nil
}
