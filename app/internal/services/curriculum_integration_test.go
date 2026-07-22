package services

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/db/migrations"
)

func TestCurriculumGraphMutations(t *testing.T) {
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
		if _, err := database.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		if err := database.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if _, err := database.Exec("SET search_path TO " + schema); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(database); err != nil {
		t.Fatal(err)
	}

	foundations := createTestUnit(t, database, "Foundations")
	algebra := createTestUnit(t, database, "Algebra")
	calculus := createTestUnit(t, database, "Calculus")
	if err := AddUnitDependency(database, algebra, foundations); err != nil {
		t.Fatal(err)
	}
	if err := AddUnitDependency(database, calculus, algebra); err != nil {
		t.Fatal(err)
	}
	if err := AddUnitDependency(database, foundations, calculus); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("closing a cycle returned %v", err)
	}
	if err := AddUnitDependency(database, algebra, foundations); !errors.Is(err, ErrDependencyExists) {
		t.Fatalf("duplicating a dependency returned %v", err)
	}

	err = DeleteCurriculumUnit(database, foundations)
	var prerequisiteError *UnitIsPrerequisiteError
	if !errors.As(err, &prerequisiteError) || len(prerequisiteError.DependentNames) != 1 {
		t.Fatalf("deleting a required prerequisite returned %v", err)
	}
	if err := RemoveUnitDependency(database, algebra, foundations); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCurriculumUnit(database, foundations); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCurriculumUnit(database, calculus); err != nil {
		t.Fatalf("deleting a unit with its own prerequisites: %v", err)
	}

	graph, err := db.GetCurriculumGraph(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Units) != 1 || graph.Units[0].ID != algebra || len(graph.Dependencies) != 0 {
		t.Fatalf("unexpected final graph: %#v", graph)
	}
}

func createTestUnit(t *testing.T, database *sql.DB, name string) int64 {
	t.Helper()
	unit, err := CreateCurriculumUnit(database, name, name+" description")
	if err != nil {
		t.Fatal(err)
	}
	return unit.ID
}
