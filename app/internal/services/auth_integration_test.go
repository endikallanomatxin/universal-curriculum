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

func TestRegisterLocalUserCreatesAuthenticatableAccountAtomically(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	schema := fmt.Sprintf("registration_test_%d", time.Now().UnixNano())
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

	registered, err := RegisterLocalUser(database, "  Ada Lovelace  ", " Ada@Example.COM ", "long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if registered.FullName != "Ada Lovelace" || registered.Email != "ada@example.com" || registered.IsAdmin {
		t.Fatalf("unexpected registered user: %#v", registered)
	}

	authenticated, err := AuthenticateLocal(database, "ADA@example.com", "long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != registered.ID {
		t.Fatalf("authenticated user = %d, want %d", authenticated.ID, registered.ID)
	}

	if _, err := RegisterLocalUser(database, "Another learner", "ada@example.com", "another-long-password"); !errors.Is(err, db.ErrEmailAlreadyRegistered) {
		t.Fatalf("duplicate registration error = %v, want %v", err, db.ErrEmailAlreadyRegistered)
	}
	var userCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatal(err)
	}
	if userCount != 1 {
		t.Fatalf("user count after duplicate registration = %d, want 1", userCount)
	}
}
