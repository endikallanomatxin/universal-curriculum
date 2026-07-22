package models

import (
	"errors"
	"strings"
	"time"
)

const (
	MinimumPasswordLength = 10
	MaximumPasswordBytes  = 72
)

var (
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

func (user User) DisplayName() string {
	if user.Alias != nil {
		return *user.Alias
	}
	return user.FullName
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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
