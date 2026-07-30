package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/db/migrations"
)

func TestRegisterLocalUserCreatesAuthenticatableAccountAtomically(t *testing.T) {
	database := openAuthIntegrationDatabase(t, "registration")

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

type recordingEmailSender struct {
	messages []EmailMessage
}

func (sender *recordingEmailSender) Send(_ context.Context, message EmailMessage) error {
	sender.messages = append(sender.messages, message)
	return nil
}

func TestPasswordRecoverySendsOneTimeLinkAndRevokesExistingSessions(t *testing.T) {
	database := openAuthIntegrationDatabase(t, "password_recovery")

	user, err := RegisterLocalUser(database, "Ada Lovelace", "ada@example.com", "old-long-password")
	if err != nil {
		t.Fatal(err)
	}
	sessionToken, err := db.CreateSession(database, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	sender := &recordingEmailSender{}
	if err := RequestPasswordReset(
		context.Background(),
		database,
		sender,
		"https://curriculum.example",
		"unknown@example.com",
	); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("messages for unknown account = %d, want 0", len(sender.messages))
	}

	if err := RequestPasswordReset(
		context.Background(),
		database,
		sender,
		"https://curriculum.example",
		" ADA@Example.com ",
	); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(sender.messages))
	}
	if sender.messages[0].To != "ada@example.com" {
		t.Fatalf("recipient = %q", sender.messages[0].To)
	}

	// A repeated request during the cooldown does not send another message or
	// replace the link already delivered.
	if err := RequestPasswordReset(
		context.Background(),
		database,
		sender,
		"https://curriculum.example",
		"ada@example.com",
	); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent messages during cooldown = %d, want 1", len(sender.messages))
	}

	resetURL, err := url.Parse(strings.TrimSpace(strings.Split(sender.messages[0].Text, "\n")[3]))
	if err != nil {
		t.Fatal(err)
	}
	resetToken := resetURL.Query().Get("token")
	if resetToken == "" {
		t.Fatal("password reset email does not contain a token")
	}
	var rawTokenCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM password_reset_tokens WHERE token_hash = $1`,
		resetToken,
	).Scan(&rawTokenCount); err != nil {
		t.Fatal(err)
	}
	if rawTokenCount != 0 {
		t.Fatal("password reset token was stored without hashing")
	}

	if err := ResetPassword(database, resetToken, "new-long-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateLocal(database, "ada@example.com", "old-long-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password authentication error = %v", err)
	}
	if _, err := AuthenticateLocal(database, "ada@example.com", "new-long-password"); err != nil {
		t.Fatalf("authenticate with reset password: %v", err)
	}
	if _, err := db.UseSession(database, sessionToken); !errors.Is(err, db.ErrSessionNotFound) {
		t.Fatalf("old session error = %v, want ErrSessionNotFound", err)
	}
	valid, err := db.PasswordResetTokenIsValid(database, resetToken)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("consumed password reset token is still valid")
	}
}

func TestPasswordRecoveryRateLimitsRequestsAndAttemptsSeparately(t *testing.T) {
	database := openAuthIntegrationDatabase(t, "password_recovery_rate_limit")
	limiter := NewPasswordResetRateLimiter(database)

	for attempt := 1; attempt <= passwordResetRequestBlockThreshold; attempt++ {
		blocked, err := limiter.RegisterRequest("198.51.100.7")
		if err != nil {
			t.Fatal(err)
		}
		wantBlocked := attempt == passwordResetRequestBlockThreshold
		if blocked != wantBlocked {
			t.Fatalf("request %d blocked = %v, want %v", attempt, blocked, wantBlocked)
		}
	}

	blocked, err := limiter.RegisterAttempt("198.51.100.7")
	if err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Fatal("request rate limit also blocked the independent reset-attempt scope")
	}
}

func openAuthIntegrationDatabase(t *testing.T, prefix string) *sql.DB {
	t.Helper()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	schema := fmt.Sprintf("%s_test_%d", prefix, time.Now().UnixNano())
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
	return database
}
