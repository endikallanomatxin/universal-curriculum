package services

import (
	"database/sql"
	"errors"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

const MaximumAPITokenNameLength = 100

var (
	ErrAPITokenNameRequired = errors.New("API token name is required")
	ErrAPITokenNameTooLong  = errors.New("API token name is too long")
	ErrAPITokenNotFound     = errors.New("API token not found")
)

func CreateAPIToken(database *sql.DB, userID int64, name string) (*models.APIToken, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrAPITokenNameRequired
	}
	if len([]rune(name)) > MaximumAPITokenNameLength {
		return nil, ErrAPITokenNameTooLong
	}
	return db.CreateAPIToken(database, userID, name)
}

func RevokeAPIToken(database *sql.DB, userID, tokenID int64) error {
	deleted, err := db.DeleteAPIToken(database, userID, tokenID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrAPITokenNotFound
	}
	return nil
}
