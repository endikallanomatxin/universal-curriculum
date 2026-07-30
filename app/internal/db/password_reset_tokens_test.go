package db

import (
	"errors"
	"testing"
)

func TestMalformedPasswordResetTokensAreRejectedBeforeDatabaseWork(t *testing.T) {
	valid, err := PasswordResetTokenIsValid(nil, "invalid-token")
	if err != nil || valid {
		t.Fatalf("PasswordResetTokenIsValid() = %v, %v", valid, err)
	}
	if err := ResetPasswordWithToken(nil, "invalid-token", []byte("unused")); !errors.Is(err, ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPasswordWithToken() error = %v, want ErrInvalidPasswordResetToken", err)
	}
}
