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
	foundations, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Foundations", "Learn the core foundations.")
	if err != nil {
		t.Fatal(err)
	}
	algebra, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Algebra", "Learn variables and equations.")
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
	learningPath, err := CreateLearningPath(
		database, authorID, "Algebra goal", "Keep a personal target while the curriculum evolves.", []int64{algebra.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetUnitCompleted(database, authorID, foundations.ID, true); err != nil {
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
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Work through variables, expressions, and equations."); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Learn variables and equations."); err != nil {
		t.Fatal(err)
	}
	draft, err := db.GetCurriculumProposal(database, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range draft.Changes {
		if change.Kind == "update_unit" || change.Kind == "update_content" {
			t.Fatalf("unchanged unit value left a proposal change behind: %#v", change)
		}
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Work through variables, expressions, and equations."); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Work through variables, expressions, and equations."); err != nil {
		t.Fatal(err)
	}
	draft, err = db.GetCurriculumProposal(database, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	changeCounts := map[string]int{}
	for _, change := range draft.Changes {
		changeCounts[change.Kind]++
	}
	if changeCounts["update_unit"] != 1 || changeCounts["update_content"] != 1 {
		t.Fatalf("unit edits accumulated duplicate proposal changes: %#v", draft.Changes)
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
	completedUnitIDs, err := db.CompletedUnitIDs(database, authorID)
	if err != nil || !completedUnitIDs[foundations.ID] {
		t.Fatalf("curriculum publication did not preserve completion: ids=%v err=%v", completedUnitIDs, err)
	}
	persistedPath, err := db.GetLearningPath(database, authorID, learningPath.ID)
	if err != nil || persistedPath == nil || len(persistedPath.Units) != 1 || persistedPath.Units[0].ID != algebra.ID {
		t.Fatalf("curriculum rebuild did not preserve the learning path: path=%#v err=%v", persistedPath, err)
	}
	if graph.Units[0].ID == algebra.ID && graph.Units[0].Content != "Work through variables, expressions, and equations." ||
		graph.Units[1].ID == algebra.ID && graph.Units[1].Content != "Work through variables, expressions, and equations." {
		t.Fatalf("content change was not published: %#v", graph.Units)
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
	for _, unit := range graph.Units {
		if unit.ID == algebra.ID && unit.Content != "Learn variables and equations." {
			t.Fatalf("revert did not restore unit content: %#v", unit)
		}
	}
	if err := db.SetUnitCompleted(database, authorID, foundations.ID, false); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || completedUnitIDs[foundations.ID] {
		t.Fatalf("unit remained completed after returning it to pending: ids=%v err=%v", completedUnitIDs, err)
	}
}
