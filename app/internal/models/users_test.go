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

func TestRegistrationIdentityValidation(t *testing.T) {
	for _, test := range []struct {
		name     string
		fullName string
		wantErr  error
	}{
		{name: "valid", fullName: "Ada Lovelace"},
		{name: "surrounding whitespace", fullName: " Ada Lovelace "},
		{name: "empty", fullName: "  ", wantErr: ErrFullNameRequired},
		{name: "too long", fullName: strings.Repeat("a", MaximumFullNameLength+1), wantErr: ErrFullNameTooLong},
	} {
		t.Run("name "+test.name, func(t *testing.T) {
			if err := ValidateFullName(test.fullName); !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidateFullName() error = %v, want %v", err, test.wantErr)
			}
		})
	}

	for _, test := range []struct {
		name    string
		email   string
		wantErr error
	}{
		{name: "valid", email: "student@example.com"},
		{name: "normalized", email: " Student@Example.COM "},
		{name: "empty", email: "", wantErr: ErrInvalidEmail},
		{name: "missing domain", email: "student", wantErr: ErrInvalidEmail},
		{name: "display address", email: "Student <student@example.com>", wantErr: ErrInvalidEmail},
		{name: "too long", email: strings.Repeat("a", MaximumEmailLength) + "@example.com", wantErr: ErrInvalidEmail},
	} {
		t.Run("email "+test.name, func(t *testing.T) {
			if err := ValidateEmail(test.email); !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidateEmail() error = %v, want %v", err, test.wantErr)
			}
		})
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
