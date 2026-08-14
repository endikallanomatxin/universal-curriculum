package services

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"universal-curriculum/internal/db/migrations"
)

var integrationSchemaPrefix = regexp.MustCompile(`[^a-z0-9_]+`)

func openPostgresIntegrationDatabase(t *testing.T, prefix string) *sql.DB {
	t.Helper()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	prefix = integrationSchemaPrefix.ReplaceAllString(prefix, "_")
	schema := fmt.Sprintf("%s_test_%d", prefix, time.Now().UnixNano())
	database, err := sql.Open("postgres", connectionString)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	if _, err := database.Exec("CREATE SCHEMA " + schema); err != nil {
		_ = database.Close()
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
	return database
}
