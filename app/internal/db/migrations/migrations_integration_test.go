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
	if err := goose.UpTo(database, "sql", 2); err != nil {
		t.Fatalf("apply pre-0.3 migrations: %v", err)
	}
	var userID int64
	if err := database.QueryRow(`INSERT INTO users (full_name) VALUES ('Migration user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("create migration fixture user: %v", err)
	}
	fixtureTx, err := database.Begin()
	if err != nil {
		t.Fatalf("begin 0.2 proposal fixtures: %v", err)
	}
	var acceptedID, rejectedID int64
	if err := fixtureTx.QueryRow(`
		INSERT INTO curriculum_proposals (title, rationale, status, created_at)
		VALUES ('Accepted proposal', 'Existing accepted work', 'draft', '2026-08-01T10:00:00Z')
		RETURNING id
	`).Scan(&acceptedID); err != nil {
		_ = fixtureTx.Rollback()
		t.Fatalf("create accepted 0.2 proposal: %v", err)
	}
	if err := fixtureTx.QueryRow(`
		INSERT INTO curriculum_proposals (title, rationale, status, created_at)
		VALUES ('Rejected proposal', 'Existing rejected work', 'draft', '2026-08-03T12:00:00Z')
		RETURNING id
	`).Scan(&rejectedID); err != nil {
		_ = fixtureTx.Rollback()
		t.Fatalf("create rejected 0.2 proposal: %v", err)
	}
	if _, err := fixtureTx.Exec(`
		INSERT INTO curriculum_proposal_authors (proposal_id, user_id)
		VALUES ($1, $3), ($2, $3)
	`, acceptedID, rejectedID, userID); err != nil {
		_ = fixtureTx.Rollback()
		t.Fatalf("create 0.2 proposal authors: %v", err)
	}
	if _, err := fixtureTx.Exec(`
		UPDATE curriculum_proposals
		SET status = 'accepted', accepted_at = '2026-08-02T11:00:00Z'
		WHERE id = $1
	`, acceptedID); err != nil {
		_ = fixtureTx.Rollback()
		t.Fatalf("accept 0.2 proposal: %v", err)
	}
	if _, err := fixtureTx.Exec(`UPDATE curriculum_projection_state SET proposal_id = $1 WHERE singleton = TRUE`, acceptedID); err != nil {
		_ = fixtureTx.Rollback()
		t.Fatalf("project accepted 0.2 proposal: %v", err)
	}
	if _, err := fixtureTx.Exec(`UPDATE curriculum_proposals SET status = 'rejected' WHERE id = $1`, rejectedID); err != nil {
		_ = fixtureTx.Rollback()
		t.Fatalf("reject 0.2 proposal: %v", err)
	}
	if err := fixtureTx.Commit(); err != nil {
		t.Fatalf("commit 0.2 proposal fixtures: %v", err)
	}
	if err := goose.Up(database, "sql"); err != nil {
		t.Fatalf("upgrade populated 0.2 schema: %v", err)
	}
	for _, test := range []struct {
		id     int64
		status string
	}{
		{id: acceptedID, status: "accepted"},
		{id: rejectedID, status: "rejected"},
	} {
		var timestampsMatch bool
		if err := database.QueryRow(`
			SELECT submitted_at = COALESCE(accepted_at, created_at)
			   AND decided_at = COALESCE(accepted_at, created_at)
			FROM curriculum_proposals
			WHERE id = $1 AND status = $2
		`, test.id, test.status).Scan(&timestampsMatch); err != nil {
			t.Fatalf("read upgraded %s proposal: %v", test.status, err)
		}
		if !timestampsMatch {
			t.Fatalf("%s proposal timestamps were not backfilled", test.status)
		}
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
