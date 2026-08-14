package services

import (
	"testing"

	"universal-curriculum/internal/db"
)

func TestDraftUnitIdentityCleanupAndAutomaticRebase(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "curriculum_draft_identity")
	authorID, currentUnitID := publishIntegrationUnit(t, database)

	discarded, err := CreateCurriculumProposal(database, authorID, "Discarded draft", "Exercise hypothetical identity cleanup.")
	if err != nil {
		t.Fatal(err)
	}
	hypothetical, err := CreateCurriculumUnit(database, authorID, discarded.ID, "Hypothetical", "Never published.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, discarded.ID, hypothetical.ID, "Edited hypothetical"); err != nil {
		t.Fatal(err)
	}
	if err := AddUnitDependency(database, authorID, discarded.ID, hypothetical.ID, currentUnitID); err != nil {
		t.Fatal(err)
	}
	if err := AddCurriculumRecognition(database, authorID, discarded.ID, []int64{currentUnitID}, []int64{hypothetical.ID}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCurriculumUnit(database, authorID, discarded.ID, hypothetical.ID); err != nil {
		t.Fatalf("discard hypothetical unit: %v", err)
	}
	discarded, err = db.GetCurriculumProposal(database, discarded.ID)
	if err != nil || discarded == nil || len(discarded.Changes) != 0 {
		t.Fatalf("discarded hypothetical unit left proposal changes: proposal=%#v err=%v", discarded, err)
	}
	var hypotheticalCreations int
	if err := database.QueryRow(`
		SELECT count(*) FROM curriculum_unit_creations WHERE change_id = $1
	`, hypothetical.ID).Scan(&hypotheticalCreations); err != nil {
		t.Fatal(err)
	}
	if hypotheticalCreations != 0 {
		t.Fatalf("discarded hypothetical unit creation still exists: %d", hypotheticalCreations)
	}
	if err := DeleteCurriculumProposal(database, authorID, discarded.ID); err != nil {
		t.Fatalf("delete draft with internally referenced hypothetical unit: %v", err)
	}

	rebasedCreation, err := CreateCurriculumProposal(database, authorID, "Rebased creation", "Preserve a created unit's final state through rebase.")
	if err != nil {
		t.Fatal(err)
	}
	rebasedUnit, err := CreateCurriculumUnit(database, authorID, rebasedCreation.ID, "Initial draft name", "Initial draft content.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitAndContent(database, authorID, rebasedCreation.ID, rebasedUnit.ID, "Rebased final name", "Rebased final content."); err != nil {
		t.Fatal(err)
	}
	upstreamRevision, err := CreateCurriculumProposal(database, authorID, "Independent upstream revision", "Trigger automatic rebase of the created unit.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, upstreamRevision.ID, currentUnitID, "An independent upstream content revision."); err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, upstreamRevision.ID); err != nil {
		t.Fatal(err)
	}
	rebasedCreation, err = db.GetCurriculumProposal(database, rebasedCreation.ID)
	if err != nil || rebasedCreation == nil || rebasedCreation.BaseProposalID == nil ||
		*rebasedCreation.BaseProposalID != upstreamRevision.ID || len(rebasedCreation.Changes) != 1 ||
		rebasedCreation.Changes[0].Kind != "create_unit" ||
		rebasedCreation.Changes[0].UnitName != "Rebased final name" ||
		rebasedCreation.Changes[0].UnitContent != "Rebased final content." {
		t.Fatalf("automatic rebase changed the final unit creation: proposal=%#v err=%v", rebasedCreation, err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, rebasedCreation.ID); err != nil {
		t.Fatal(err)
	}
	publishedRebasedUnit, err := db.GetUnit(database, rebasedUnit.ID)
	if err != nil || publishedRebasedUnit == nil || publishedRebasedUnit.Name != "Rebased final name" || publishedRebasedUnit.Content != "Rebased final content." {
		t.Fatalf("published rebased unit = %#v err=%v", publishedRebasedUnit, err)
	}
}
