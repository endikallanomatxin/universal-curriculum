package services

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/db/migrations"
)

func TestCurriculumProposalCollectsChangesAndPublishesAtomically(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	schema := fmt.Sprintf("curriculum_test_%d", time.Now().UnixNano())
	database, err := sql.Open("postgres", connectionString)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	if _, err := database.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
		_ = database.Close()
	})
	if _, err := database.Exec("SET search_path TO " + schema); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(database); err != nil {
		t.Fatal(err)
	}
	var authorID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name, is_admin) VALUES ('Curriculum Editor', TRUE) RETURNING id
	`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}

	proposal, err := CreateCurriculumProposal(database, authorID, "Mathematics foundations", "Introduce a coherent learning path.")
	if err != nil {
		t.Fatal(err)
	}
	foundations, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Foundations", "Core foundations")
	if err != nil {
		t.Fatal(err)
	}
	algebra, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Algebra", "Core algebra")
	if err != nil {
		t.Fatal(err)
	}
	// Draft changes do not mutate the published projection.
	graph, err := db.GetCurriculumGraph(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Units) != 0 {
		t.Fatalf("draft leaked into projection: %#v", graph)
	}

	// Publish the unit creations first: later proposals can refer to their stable IDs.
	if err := PublishCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	proposal, err = CreateCurriculumProposal(database, authorID, "Algebra path", "Connect and refine the new units.")
	if err != nil {
		t.Fatal(err)
	}
	if err := AddUnitDependency(database, authorID, proposal.ID, algebra.ID, foundations.ID); err != nil {
		t.Fatal(err)
	}
	if err := RemoveUnitDependency(database, authorID, proposal.ID, algebra.ID, foundations.ID); err != nil {
		t.Fatalf("remove dependency staged in the same proposal: %v", err)
	}
	if err := AddUnitDependency(database, authorID, proposal.ID, algebra.ID, foundations.ID); err != nil {
		t.Fatalf("restore dependency staged in the same proposal: %v", err)
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra", "Variables and equations"); err != nil {
		t.Fatal(err)
	}
	if err := PublishCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	graph, err = db.GetCurriculumGraph(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Units) != 2 || len(graph.Dependencies) != 1 {
		t.Fatalf("unexpected published graph: %#v", graph)
	}
	if err := RevertCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	graph, err = db.GetCurriculumGraph(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Dependencies) != 0 {
		t.Fatalf("revert did not undo whole proposal: %#v", graph)
	}
}
