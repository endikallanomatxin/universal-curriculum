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
	other, err := db.CreateLocalUser(database, "Other contributor", "other@example.com", []byte("hash"))
	if err != nil {
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
	if visible, err := GetVisibleCurriculumProposal(database, other.ID, false, proposal.ID); err != nil || visible.Status != "submitted" {
		t.Fatalf("active proposal visibility = %#v, %v", visible, err)
	}
	if _, err := CreateCurriculumProposal(database, other.ID, "Newer private draft", "Must not displace an active proposal."); err != nil {
		t.Fatal(err)
	}
	active, total, err := db.ListSubmittedCurriculumProposals(database, 1, 0)
	if err != nil || total != 1 || len(active) != 1 || active[0].ID != proposal.ID {
		t.Fatalf("active proposals = %#v, total %d, err %v", active, total, err)
	}
	if err := RejectCurriculumProposal(database, proposal.ID); err != nil {
		t.Fatal(err)
	}

	stored, err := GetVisibleCurriculumProposal(database, author.ID, false, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "rejected" || len(stored.Changes) != 1 {
		t.Fatalf("rejected proposal = %#v", stored)
	}
	if _, err := GetEditableCurriculumProposal(database, author.ID, proposal.ID); err != ErrProposalNotFound {
		t.Fatalf("edit rejected proposal error = %v", err)
	}
	authored, total, err := db.ListCurriculumProposalsByAuthor(database, author.ID, 1, 0)
	if err != nil || total != 1 || len(authored) != 1 || authored[0].ID != proposal.ID {
		t.Fatalf("authored proposals = %#v, total %d, err %v", authored, total, err)
	}
}

func TestSubmittedProposalRequiresAdministratorDecision(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "proposal_acceptance")
	author, _ := db.CreateLocalUser(database, "Contributor", "contributor@example.com", []byte("hash"))
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
	if _, err := AcceptCurriculumProposal(database, proposal.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := db.GetCurrentCurriculumProposalID(database)
	if after == nil || *after != proposal.ID {
		t.Fatalf("current proposal = %v, want %d", after, proposal.ID)
	}
}

func TestAcceptAppliesOnlyPublishedProposalAndLeavesOtherDraftsLazy(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "incremental_proposal_acceptance")
	author, _ := db.CreateLocalUser(database, "Contributor", "incremental@example.com", []byte("hash"))

	base, err := CreateCurriculumProposal(database, author.ID, "Base", "Create the published base.")
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := CreateCurriculumUnit(database, author.ID, base.ID, "Unchanged", "Original content")
	if err != nil {
		t.Fatal(err)
	}
	if err := SubmitCurriculumProposal(database, author.ID, base.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptCurriculumProposal(database, base.ID); err != nil {
		t.Fatal(err)
	}
	var unchangedUpdatedAt string
	if err := database.QueryRow(`SELECT updated_at::TEXT FROM units WHERE id = $1`, unchanged.ID).Scan(&unchangedUpdatedAt); err != nil {
		t.Fatal(err)
	}

	staleDraft, err := CreateCurriculumProposal(database, author.ID, "Lazy draft", "Remain stale until it is used.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCurriculumUnit(database, author.ID, staleDraft.ID, "Draft unit", "Draft content"); err != nil {
		t.Fatal(err)
	}

	next, err := CreateCurriculumProposal(database, author.ID, "Next", "Advance the curriculum independently.")
	if err != nil {
		t.Fatal(err)
	}
	created, err := CreateCurriculumUnit(database, author.ID, next.ID, "New unit", "New content")
	if err != nil {
		t.Fatal(err)
	}
	if err := SubmitCurriculumProposal(database, author.ID, next.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptCurriculumProposal(database, next.ID); err != nil {
		t.Fatal(err)
	}

	var storedUpdatedAt string
	if err := database.QueryRow(`SELECT updated_at::TEXT FROM units WHERE id = $1`, unchanged.ID).Scan(&storedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if storedUpdatedAt != unchangedUpdatedAt {
		t.Fatalf("unchanged unit timestamp changed from %s to %s", unchangedUpdatedAt, storedUpdatedAt)
	}
	if unit, err := db.GetUnit(database, created.ID); err != nil || unit == nil || unit.Content != "New content" {
		t.Fatalf("incrementally projected unit = %#v, err=%v", unit, err)
	}
	storedDraft, err := db.GetCurriculumProposal(database, staleDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedDraft.BaseProposalID == nil || *storedDraft.BaseProposalID != base.ID {
		t.Fatalf("draft was eagerly rebased to %#v", storedDraft.BaseProposalID)
	}
}
