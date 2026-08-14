package services

import (
	"database/sql"
	"testing"
)

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
	if _, err := PublishCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	return authorID, unit.ID
}
