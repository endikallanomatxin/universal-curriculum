package services

import (
	"database/sql"
	"testing"
)

func submitAndAcceptCurriculumProposal(database *sql.DB, authorID, proposalID int64) (CurriculumProposalRebaseSummary, error) {
	if err := SubmitCurriculumProposal(database, authorID, proposalID); err != nil {
		return CurriculumProposalRebaseSummary{}, err
	}
	return AcceptCurriculumProposal(database, proposalID)
}

func publishIntegrationUnit(t *testing.T, database *sql.DB) (int64, int64) {
	t.Helper()
	var authorID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name, is_admin) VALUES ('Curriculum Editor', TRUE) RETURNING id
	`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	proposal, err := CreateCurriculumProposal(database, authorID, "Initial curriculum", "Provide a published unit for the scenario.")
	if err != nil {
		t.Fatal(err)
	}
	unit, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Foundations", "Learn the foundations.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	return authorID, unit.ID
}
