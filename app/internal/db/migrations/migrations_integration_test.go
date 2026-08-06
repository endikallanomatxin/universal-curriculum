package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func TestMigrationsRoundTrip(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL migration integration tests")
	}

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	database, err := sql.Open("postgres", connectionString)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)

	if _, err := database.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create test schema: %v", err)
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
		t.Fatalf("select test schema: %v", err)
	}

	if err := setup(); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := goose.DownTo(database, "sql", 0); err != nil {
		t.Fatalf("roll back migrations: %v", err)
	}
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}
}
