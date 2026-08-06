package services

import (
	"strings"
	"testing"
)

func TestPasswordResetURLUsesConfiguredOriginAndEscapesToken(t *testing.T) {
	got := PasswordResetURL("https://curriculum.example", "secret+/=?")
	want := "https://curriculum.example/auth/reset-password?token=secret%2B%2F%3D%3F"
	if got != want {
		t.Fatalf("PasswordResetURL() = %q, want %q", got, want)
	}
}

func TestPasswordResetEmailContainsExpiringLinkAndOpaqueIdempotencyKey(t *testing.T) {
	const token = "raw-one-time-secret"
	message := passwordResetEmail(
		"learner@example.test",
		"https://curriculum.example/auth/reset-password?token=secret&other=value",
		token,
	)
	if message.To != "learner@example.test" ||
		!strings.Contains(message.Text, "expires in one hour") ||
		!strings.Contains(message.HTML, "secret&amp;other=value") ||
		message.IdempotencyKey == "" ||
		strings.Contains(message.IdempotencyKey, token) {
		t.Fatalf("password reset message = %+v", message)
	}
}
