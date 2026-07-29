package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"universal-curriculum/internal/models"
)

var (
	ErrBootstrapAdminConflict = errors.New("users already exist but the bootstrap administrator does not")
	ErrEmailAlreadyRegistered = errors.New("email is already registered")
)

func CreateLocalUser(database *sql.DB, fullName, email string, passwordHash []byte) (*models.User, error) {
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin local user transaction: %w", err)
	}
	defer tx.Rollback()

	var user models.User
	var alias sql.NullString
	err = tx.QueryRow(`
		INSERT INTO users (full_name)
		VALUES ($1)
		RETURNING id, full_name, alias, is_admin, created_at, updated_at
	`, strings.TrimSpace(fullName)).Scan(
		&user.ID, &user.FullName, &alias, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create local user: %w", err)
	}

	email = models.NormalizeEmail(email)
	if _, err := tx.Exec(`
		INSERT INTO local_authentications (user_id, email, password_hash)
		VALUES ($1, $2, $3)
	`, user.ID, email, passwordHash); err != nil {
		var postgresError *pq.Error
		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" &&
			postgresError.Constraint == "local_authentications_email_unique" {
			return nil, ErrEmailAlreadyRegistered
		}
		return nil, fmt.Errorf("create local user credentials: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit local user: %w", err)
	}

	user.Email = email
	user.Alias = nullStringPointer(alias)
	return &user, nil
}

func CreateBootstrapAdmin(database *sql.DB, fullName, alias, email string, passwordHash []byte) (*models.User, error) {
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin bootstrap administrator transaction: %w", err)
	}
	defer tx.Rollback()

	email = models.NormalizeEmail(email)
	existing, err := getUserByEmail(tx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !existing.IsAdmin {
			return nil, fmt.Errorf("bootstrap email belongs to a non-administrator")
		}
		return existing, nil
	}

	var userCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if userCount != 0 {
		return nil, ErrBootstrapAdminConflict
	}

	fullName = strings.TrimSpace(fullName)
	alias = strings.TrimSpace(alias)
	var user models.User
	var storedAlias sql.NullString
	err = tx.QueryRow(`
		INSERT INTO users (full_name, alias, is_admin)
		VALUES ($1, NULLIF($2, ''), TRUE)
		RETURNING id, full_name, alias, is_admin, created_at, updated_at
	`, fullName, alias).Scan(&user.ID, &user.FullName, &storedAlias, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create bootstrap administrator: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO local_authentications (user_id, email, password_hash)
		VALUES ($1, $2, $3)
	`, user.ID, email, passwordHash); err != nil {
		return nil, fmt.Errorf("create bootstrap administrator credentials: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bootstrap administrator: %w", err)
	}
	user.Email = email
	user.Alias = nullStringPointer(storedAlias)
	return &user, nil
}

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func getUserByEmail(database rowQuerier, email string) (*models.User, error) {
	var user models.User
	var alias sql.NullString
	err := database.QueryRow(`
		SELECT u.id, u.full_name, u.alias, la.email, u.is_admin, u.created_at, u.updated_at
		FROM users u
		JOIN local_authentications la ON la.user_id = u.id
		WHERE la.email = $1
	`, models.NormalizeEmail(email)).Scan(
		&user.ID, &user.FullName, &alias, &user.Email, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	user.Alias = nullStringPointer(alias)
	return &user, nil
}

func GetUserByID(database *sql.DB, userID int64) (*models.User, error) {
	var user models.User
	var alias sql.NullString
	err := database.QueryRow(`
		SELECT u.id, u.full_name, u.alias, la.email, u.is_admin, u.created_at, u.updated_at
		FROM users u
		JOIN local_authentications la ON la.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&user.ID, &user.FullName, &alias, &user.Email, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	user.Alias = nullStringPointer(alias)
	return &user, nil
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func GetLocalPasswordHash(database *sql.DB, email string) (int64, []byte, error) {
	var userID int64
	var passwordHash []byte
	err := database.QueryRow(`
		SELECT user_id, password_hash
		FROM local_authentications
		WHERE email = $1
	`, models.NormalizeEmail(email)).Scan(&userID, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, fmt.Errorf("get local authentication: %w", err)
	}
	return userID, passwordHash, nil
}
