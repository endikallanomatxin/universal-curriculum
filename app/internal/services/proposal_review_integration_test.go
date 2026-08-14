package services

import (
	"testing"

	"universal-curriculum/internal/db"
)

func TestRejectedProposalIsPreservedAndReadableByItsAuthor(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "proposal_review")
	author, err := db.CreateLocalUser(database, "Contributor", "contributor@example.com", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := db.CreateLocalUser(database, "Administrator", "admin@example.com", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE users SET is_contributor = TRUE WHERE id = $1`, author.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE users SET is_admin = TRUE WHERE id = $1`, administrator.ID); err != nil {
		t.Fatal(err)
	}
	proposal, err := CreateCurriculumProposal(database, author.ID, "Preserved work", "Keep useful rejected work available.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCurriculumUnit(database, author.ID, proposal.ID, "Proposed unit", "Useful content"); err != nil {
		t.Fatal(err)
	}
	if err := SubmitCurriculumProposal(database, author.ID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	if err := RejectCurriculumProposal(database, administrator.ID, proposal.ID, "Needs a narrower scope."); err != nil {
		t.Fatal(err)
	}

	stored, err := GetVisibleCurriculumProposal(database, author.ID, false, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "rejected" || stored.RejectionReason != "Needs a narrower scope." || len(stored.Changes) != 1 {
		t.Fatalf("rejected proposal = %#v", stored)
	}
	if _, err := GetEditableCurriculumProposal(database, author.ID, proposal.ID); err != ErrProposalNotFound {
		t.Fatalf("edit rejected proposal error = %v", err)
	}
}

func TestSubmittedProposalRequiresAdministratorDecision(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "proposal_acceptance")
	author, _ := db.CreateLocalUser(database, "Contributor", "contributor@example.com", []byte("hash"))
	administrator, _ := db.CreateLocalUser(database, "Administrator", "admin@example.com", []byte("hash"))
	proposal, err := CreateCurriculumProposal(database, author.ID, "Accepted work", "Add a useful unit.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCurriculumUnit(database, author.ID, proposal.ID, "Accepted unit", "Accepted content"); err != nil {
		t.Fatal(err)
	}
	if err := SubmitCurriculumProposal(database, author.ID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := db.GetCurrentCurriculumProposalID(database)
	if before != nil {
		t.Fatalf("curriculum changed on submission: %d", *before)
	}
	if _, err := AcceptCurriculumProposal(database, administrator.ID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := db.GetCurrentCurriculumProposalID(database)
	if after == nil || *after != proposal.ID {
		t.Fatalf("current proposal = %v, want %d", after, proposal.ID)
	}
}
