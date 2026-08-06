package models

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

const (
	MinimumPasswordLength = 10
	MaximumPasswordBytes  = 72
	MaximumFullNameLength = 200
	MaximumEmailLength    = 320
)

var (
	ErrFullNameRequired = errors.New("full name is required")
	ErrFullNameTooLong  = errors.New("full name is too long")
	ErrInvalidEmail     = errors.New("email is invalid")
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooLong  = errors.New("password is too long")
)

type User struct {
	ID        int64
	FullName  string
	Alias     *string
	Email     string
	IsAdmin   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type APIToken struct {
	ID         int64
	UserID     int64
	Name       string
	Prefix     string
	Token      string
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

type PasswordResetToken struct {
	// Token is the raw one-time secret returned only when it is created.
	// Persistence stores its hash, never this value.
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

func (user User) DisplayName() string {
	if user.Alias != nil {
		return *user.Alias
	}
	return user.FullName
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidateFullName(fullName string) error {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return ErrFullNameRequired
	}
	if len([]rune(fullName)) > MaximumFullNameLength {
		return ErrFullNameTooLong
	}
	return nil
}

func ValidateEmail(email string) error {
	email = NormalizeEmail(email)
	if email == "" || len(email) > MaximumEmailLength {
		return ErrInvalidEmail
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return ErrInvalidEmail
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < MinimumPasswordLength {
		return ErrPasswordTooShort
	}
	if len([]byte(password)) > MaximumPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}
