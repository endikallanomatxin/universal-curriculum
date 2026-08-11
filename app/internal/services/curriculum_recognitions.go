package services

import (
	"database/sql"
	"errors"
	"fmt"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

func AddCurriculumRecognition(
	database *sql.DB,
	authorID, proposalID int64,
	sourceUnitIDs, targetUnitIDs []int64,
) error {
	sourceUnitIDs = uniquePositiveIDs(sourceUnitIDs)
	targetUnitIDs = uniquePositiveIDs(targetUnitIDs)
	if len(sourceUnitIDs) == 0 {
		return ErrRecognitionSourcesRequired
	}
	if len(targetUnitIDs) == 0 {
		return ErrRecognitionTargetsRequired
	}
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
	base, err := db.GetCurriculumGraph(tx)
	if err != nil {
		return err
	}
	result := CurriculumGraphWithProposal(base, proposal)
	recognition := &models.Recognition{}
	for _, unitID := range sourceUnitIDs {
		unit := curriculumUnitByID(base, unitID)
		if unit == nil {
			return ErrUnitNotFound
		}
		recognition.Sources = append(recognition.Sources, *unit)
	}
	for _, unitID := range targetUnitIDs {
		unit := curriculumUnitByID(result, unitID)
		if unit == nil {
			return ErrUnitNotFound
		}
		recognition.Targets = append(recognition.Targets, *unit)
	}
	if err := db.AddDraftCurriculumProposalChange(tx, proposalID, authorID, &models.CurriculumProposalChange{
		Kind: "recognition", Recognition: recognition,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProposalNotFound
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit curriculum recognition: %w", err)
	}
	return nil
}

// EnsureCurriculumRecognition adds a recognition only when the draft does not
// already contain the same source and target sets. It gives retrying adapters
// an idempotent application operation without changing the lower-level add
// semantics used by the web and REST interfaces.
func EnsureCurriculumRecognition(
	database *sql.DB,
	authorID, proposalID int64,
	sourceUnitIDs, targetUnitIDs []int64,
) error {
	sources := uniquePositiveIDs(sourceUnitIDs)
	targets := uniquePositiveIDs(targetUnitIDs)
	if len(sources) == 0 {
		return ErrRecognitionSourcesRequired
	}
	if len(targets) == 0 {
		return ErrRecognitionTargetsRequired
	}
	if err := EnsureCurriculumProposalReady(database, authorID, proposalID); err != nil {
		return err
	}
	proposal, err := db.GetCurriculumProposal(database, proposalID)
	if err != nil {
		return err
	}
	if proposal == nil || proposal.Status != "draft" || !proposal.HasAuthor(authorID) {
		return ErrProposalNotFound
	}
	for _, change := range proposal.Changes {
		if sameRecognitionUnitIDs(change.Recognition, sources, targets) {
			return nil
		}
	}
	return AddCurriculumRecognition(database, authorID, proposalID, sources, targets)
}

func sameRecognitionUnitIDs(recognition *models.Recognition, sourceIDs, targetIDs []int64) bool {
	if recognition == nil || len(recognition.Sources) != len(sourceIDs) || len(recognition.Targets) != len(targetIDs) {
		return false
	}
	sources := make(map[int64]bool, len(sourceIDs))
	for _, id := range sourceIDs {
		sources[id] = true
	}
	for _, unit := range recognition.Sources {
		if !sources[unit.ID] {
			return false
		}
	}
	targets := make(map[int64]bool, len(targetIDs))
	for _, id := range targetIDs {
		targets[id] = true
	}
	for _, unit := range recognition.Targets {
		if !targets[unit.ID] {
			return false
		}
	}
	return true
}

func uniquePositiveIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	return unique
}

func recognitionContainsUnit(
	recognition *models.Recognition,
	unitID int64,
	includeSources, includeTargets bool,
) bool {
	if recognition == nil {
		return false
	}
	if includeSources {
		for _, source := range recognition.Sources {
			if source.ID == unitID {
				return true
			}
		}
	}
	if includeTargets {
		for _, target := range recognition.Targets {
			if target.ID == unitID {
				return true
			}
		}
	}
	return false
}
