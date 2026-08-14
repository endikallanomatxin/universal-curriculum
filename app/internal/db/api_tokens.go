package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"universal-curriculum/internal/models"
)

const apiTokenMarker = "uc_api_"

func CreateAPIToken(database *sql.DB, userID int64, name string) (*models.APIToken, error) {
	secret, err := randomURLToken()
	if err != nil {
		return nil, err
	}
	raw := apiTokenMarker + secret
	token := &models.APIToken{
		UserID: userID,
		Name:   strings.TrimSpace(name),
		Prefix: apiTokenMarker + secret[:8],
		Token:  raw,
	}
	err = database.QueryRow(`
		INSERT INTO api_tokens (user_id, name, token_hash, token_prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, token.UserID, token.Name, hashAPIToken(raw), token.Prefix).Scan(
		&token.ID, &token.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create API token: %w", err)
	}
	return token, nil
}

func ListAPITokens(database *sql.DB, userID int64) ([]models.APIToken, error) {
	rows, err := database.Query(`
		SELECT id, user_id, name, token_prefix, last_used_at, created_at
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}
	defer rows.Close()
	var tokens []models.APIToken
	for rows.Next() {
		var token models.APIToken
		var lastUsedAt sql.NullTime
		if err := rows.Scan(
			&token.ID, &token.UserID, &token.Name, &token.Prefix,
			&lastUsedAt, &token.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan API token: %w", err)
		}
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate API tokens: %w", err)
	}
	return tokens, nil
}

func DeleteAPIToken(database *sql.DB, userID, tokenID int64) (bool, error) {
	result, err := database.Exec(`
		DELETE FROM api_tokens
		WHERE id = $1 AND user_id = $2
	`, tokenID, userID)
	if err != nil {
		return false, fmt.Errorf("delete API token: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func AuthenticateAPIToken(database *sql.DB, raw string) (*models.User, error) {
	if !validAPIToken(raw) {
		return nil, nil
	}
	var user models.User
	var alias sql.NullString
	err := database.QueryRow(`
		SELECT users.id, users.full_name, users.alias, authentication.email,
		       users.is_admin, users.is_contributor, users.created_at, users.updated_at
		FROM api_tokens token
		JOIN users ON users.id = token.user_id
		JOIN local_authentications authentication ON authentication.user_id = users.id
		WHERE token.token_hash = $1
	`, hashAPIToken(raw)).Scan(
		&user.ID, &user.FullName, &alias, &user.Email,
		&user.IsAdmin, &user.IsContributor, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate API token: %w", err)
	}
	user.Alias = nullStringPointer(alias)
	_, _ = database.Exec(`
		UPDATE api_tokens
		SET last_used_at = clock_timestamp()
		WHERE token_hash = $1
		  AND (last_used_at IS NULL OR last_used_at < clock_timestamp() - INTERVAL '15 minutes')
	`, hashAPIToken(raw))
	return &user, nil
}

func hashAPIToken(token string) string {
	return hashTokenSecret(token)
}

func validAPIToken(token string) bool {
	if !strings.HasPrefix(token, apiTokenMarker) {
		return false
	}
	secret := strings.TrimPrefix(token, apiTokenMarker)
	if len(secret) != 43 {
		return false
	}
	for _, character := range secret {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}
