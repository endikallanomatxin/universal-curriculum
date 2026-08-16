package services

import (
	"database/sql"
	"fmt"

	"universal-curriculum/internal/db"
)

func SubmitCurriculumProposal(
	database *sql.DB,
	authorID, proposalID int64,
) error {
	tx, proposal, err := beginDraftMutation(database, authorID, proposalID)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(proposal.Changes) == 0 {
		return ErrProposalEmpty
	}
	graph, err := db.GetCurriculumGraphWithContent(tx)
	if err != nil {
		return err
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

func AcceptCurriculumProposal(database *sql.DB, proposalID int64) (CurriculumProposalRebaseSummary, error) {
	var summary CurriculumProposalRebaseSummary
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return summary, err
	}
	defer tx.Rollback()
	currentProposalID, err := db.LockCurrentCurriculumProposal(tx)
	if err != nil {
		return summary, err
	}
	locked, err := db.LockCurriculumProposal(tx, proposalID)
	if err != nil {
		return summary, err
	}
	if !locked {
		return summary, ErrProposalNotFound
	}
	proposal, err := db.GetCurriculumProposal(tx, proposalID)
	if err != nil {
		return summary, err
	}
	if proposal == nil || proposal.Status != "submitted" {
		return summary, ErrProposalNotFound
	}
	if !sameOptionalID(proposal.BaseProposalID, currentProposalID) {
		return summary, ErrProposalOutdated
	}
	graph, err := db.GetCurriculumGraphWithContent(tx)
	if err != nil {
		return summary, err
	}
	if err := validateCurriculumProposal(graph, proposal); err != nil {
		return summary, err
	}
	ok, err := db.AcceptSubmittedCurriculumProposal(tx, proposalID)
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

func RejectCurriculumProposal(database *sql.DB, proposalID int64) error {
	tx, err := beginCurriculumProposal(database)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	locked, err := db.LockCurriculumProposal(tx, proposalID)
	if err != nil {
		return err
	}
	if !locked {
		return ErrProposalNotFound
	}
	ok, err := db.RejectSubmittedCurriculumProposal(tx, proposalID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrProposalNotFound
	}
	return tx.Commit()
}
