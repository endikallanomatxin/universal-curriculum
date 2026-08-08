package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
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
	if err := goose.UpTo(database, "sql", 3); err != nil {
		t.Fatalf("apply migrations through OAuth tokens: %v", err)
	}
	var userID int64
	if err := database.QueryRow(`INSERT INTO users (full_name) VALUES ('Migration user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("create migration fixture user: %v", err)
	}
	clientID := "https://client.example/metadata.json"
	if _, err := database.Exec(`
		INSERT INTO oauth_access_tokens (token_hash, user_id, client_id, resource, scope, expires_at)
		VALUES ($1, $2, $3, 'https://curriculum.example/mcp', 'mcp', clock_timestamp() + INTERVAL '1 hour')
	`, strings.Repeat("a", 64), userID, clientID); err != nil {
		t.Fatalf("create access-token fixture: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO oauth_refresh_tokens (token_hash, user_id, client_id, resource, scope, expires_at)
		VALUES ($1, $2, $3, 'https://curriculum.example/mcp', 'mcp', clock_timestamp() + INTERVAL '1 day')
	`, strings.Repeat("b", 64), userID, clientID); err != nil {
		t.Fatalf("create refresh-token fixture: %v", err)
	}
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("migrate OAuth connections: %v", err)
	}
	var connections, linkedTokens int
	if err := database.QueryRow(`SELECT count(*) FROM oauth_connections WHERE user_id = $1`, userID).Scan(&connections); err != nil {
		t.Fatalf("count migrated connections: %v", err)
	}
	if err := database.QueryRow(`
		SELECT (SELECT count(*) FROM oauth_access_tokens WHERE connection_id IS NOT NULL) +
		       (SELECT count(*) FROM oauth_refresh_tokens WHERE connection_id IS NOT NULL)
	`).Scan(&linkedTokens); err != nil {
		t.Fatalf("count migrated tokens: %v", err)
	}
	if connections != 1 || linkedTokens != 2 {
		t.Fatalf("migrated connections = %d, linked tokens = %d", connections, linkedTokens)
	}
	if err := goose.DownTo(database, "sql", 0); err != nil {
		t.Fatalf("roll back migrations: %v", err)
	}
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}
}
