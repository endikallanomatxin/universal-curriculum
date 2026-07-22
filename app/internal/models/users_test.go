package models

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	for _, test := range []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "valid", password: "long-enough-password"},
		{name: "too short", password: "short", wantErr: ErrPasswordTooShort},
		{name: "too long", password: strings.Repeat("x", MaximumPasswordBytes+1), wantErr: ErrPasswordTooLong},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidatePassword(test.password); !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidatePassword() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Person@Example.COM "); got != "person@example.com" {
		t.Fatalf("NormalizeEmail() = %q", got)
	}
}

func TestUserDisplayNamePrefersAlias(t *testing.T) {
	alias := "learner"
	if got := (User{FullName: "Full Name", Alias: &alias}).DisplayName(); got != alias {
		t.Fatalf("DisplayName() = %q", got)
	}
	if got := (User{FullName: "Full Name"}).DisplayName(); got != "Full Name" {
		t.Fatalf("DisplayName() fallback = %q", got)
	}
}
