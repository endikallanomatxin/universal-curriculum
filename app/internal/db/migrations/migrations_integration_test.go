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
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	var userID int64
	if err := database.QueryRow(`INSERT INTO users (full_name) VALUES ('Migration user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("create migration fixture user: %v", err)
	}
	clientID := "https://client.example/metadata.json"
	var connectionID int64
	if _, err := database.Exec(`
		INSERT INTO oauth_authorization_codes (
			code_hash, user_id, client_id, client_name, redirect_uri, resource,
			scope, code_challenge, expires_at
		)
		VALUES (
			$1, $2, $3, 'Example client', 'https://client.example/callback',
			'https://curriculum.example/mcp', 'mcp', $4,
			clock_timestamp() + INTERVAL '5 minutes'
		)
	`, strings.Repeat("c", 64), userID, clientID, strings.Repeat("v", 43)); err != nil {
		t.Fatalf("create authorization-code fixture: %v", err)
	}
	if err := database.QueryRow(`
		INSERT INTO oauth_connections (user_id, client_id, client_name, resource)
		VALUES ($1, $2, 'Example client', 'https://curriculum.example/mcp')
		RETURNING id
	`, userID, clientID).Scan(&connectionID); err != nil {
		t.Fatalf("create connection fixture: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO oauth_access_tokens (token_hash, connection_id, scope, expires_at)
		VALUES ($1, $2, 'mcp', clock_timestamp() + INTERVAL '1 hour')
	`, strings.Repeat("a", 64), connectionID); err != nil {
		t.Fatalf("create access-token fixture: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO oauth_refresh_tokens (token_hash, connection_id, scope, expires_at)
		VALUES ($1, $2, 'mcp', clock_timestamp() + INTERVAL '1 day')
	`, strings.Repeat("b", 64), connectionID); err != nil {
		t.Fatalf("create refresh-token fixture: %v", err)
	}
	var linkedTokens int
	if err := database.QueryRow(`
		SELECT (SELECT count(*) FROM oauth_access_tokens WHERE connection_id = $1) +
		       (SELECT count(*) FROM oauth_refresh_tokens WHERE connection_id = $1)
	`, connectionID).Scan(&linkedTokens); err != nil {
		t.Fatalf("count linked tokens: %v", err)
	}
	if linkedTokens != 2 {
		t.Fatalf("linked tokens = %d", linkedTokens)
	}
	if err := goose.DownTo(database, "sql", 0); err != nil {
		t.Fatalf("roll back migrations: %v", err)
	}
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}
}
