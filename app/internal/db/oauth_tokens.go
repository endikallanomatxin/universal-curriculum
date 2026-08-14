package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"universal-curriculum/internal/models"
)

const (
	oauthAuthorizationCodeMarker = "uc_oac_"
	oauthAccessTokenMarker       = "uc_oat_"
	oauthRefreshTokenMarker      = "uc_ort_"
)

func CreateOAuthAuthorizationCode(
	database *sql.DB, grant models.OAuthAuthorizationGrant,
) (string, error) {
	cleanupExpiredOAuthTokens(database)
	secret, err := randomURLToken()
	if err != nil {
		return "", err
	}
	raw := oauthAuthorizationCodeMarker + secret
	if _, err := database.Exec(`
		INSERT INTO oauth_authorization_codes (
			code_hash, user_id, client_id, client_name, redirect_uri, resource, scope,
			code_challenge, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, clock_timestamp() + INTERVAL '5 minutes')
	`, hashTokenSecret(raw), grant.UserID, grant.ClientID, grant.ClientName,
		grant.RedirectURI, grant.Resource, grant.Scope, grant.CodeChallenge); err != nil {
		return "", fmt.Errorf("create OAuth authorization code: %w", err)
	}
	return raw, nil
}

func ExchangeOAuthAuthorizationCode(
	database *sql.DB, rawCode, clientID, redirectURI, resource, codeChallenge string,
) (*models.OAuthTokenPair, error) {
	if !validOAuthSecret(rawCode, oauthAuthorizationCodeMarker) {
		return nil, nil
	}
	accessToken, refreshToken, err := newOAuthTokenPair()
	if err != nil {
		return nil, err
	}
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin OAuth code exchange: %w", err)
	}
	defer tx.Rollback()
	var grant models.OAuthAuthorizationGrant
	err = tx.QueryRow(`
		DELETE FROM oauth_authorization_codes
		WHERE code_hash = $1
		  AND client_id = $2
		  AND redirect_uri = $3
		  AND resource = $4
		  AND code_challenge = $5
		  AND expires_at > clock_timestamp()
		RETURNING user_id, client_id, client_name, redirect_uri, resource, scope, code_challenge
	`, hashTokenSecret(rawCode), clientID, redirectURI, resource, codeChallenge).Scan(
		&grant.UserID, &grant.ClientID, &grant.ClientName, &grant.RedirectURI, &grant.Resource,
		&grant.Scope, &grant.CodeChallenge,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume OAuth authorization code: %w", err)
	}
	cleanupExpiredOAuthTokens(tx)
	var connectionID int64
	err = tx.QueryRow(`
		INSERT INTO oauth_connections (user_id, client_id, client_name, resource)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, client_id, resource) DO UPDATE
		SET client_name = EXCLUDED.client_name,
		    authorized_at = clock_timestamp(),
		    last_used_at = NULL
		RETURNING id
	`, grant.UserID, grant.ClientID, grant.ClientName, grant.Resource).Scan(&connectionID)
	if err != nil {
		return nil, fmt.Errorf("create OAuth connection: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM oauth_access_tokens WHERE connection_id = $1`, connectionID); err != nil {
		return nil, fmt.Errorf("replace OAuth access tokens: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM oauth_refresh_tokens WHERE connection_id = $1`, connectionID); err != nil {
		return nil, fmt.Errorf("replace OAuth refresh tokens: %w", err)
	}
	pair := &models.OAuthTokenPair{
		AccessToken: accessToken, RefreshToken: refreshToken,
		ExpiresIn: int(time.Hour.Seconds()), Scope: grant.Scope,
	}
	if err := insertOAuthTokens(tx, connectionID, grant.Scope, pair); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit OAuth code exchange: %w", err)
	}
	return pair, nil
}

func RefreshOAuthAccessToken(
	database *sql.DB, rawRefreshToken, clientID, resource string,
) (*models.OAuthTokenPair, error) {
	if !validOAuthSecret(rawRefreshToken, oauthRefreshTokenMarker) {
		return nil, nil
	}
	accessToken, refreshToken, err := newOAuthTokenPair()
	if err != nil {
		return nil, err
	}
	tx, err := database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin OAuth token refresh: %w", err)
	}
	defer tx.Rollback()
	cleanupExpiredOAuthTokens(tx)
	var connectionID int64
	var scope string
	err = tx.QueryRow(`
		DELETE FROM oauth_refresh_tokens token
		USING oauth_connections connection
		WHERE token.connection_id = connection.id
		  AND token.token_hash = $1 AND connection.client_id = $2 AND connection.resource = $3
		  AND expires_at > clock_timestamp()
		RETURNING token.connection_id, token.scope
	`, hashTokenSecret(rawRefreshToken), clientID, resource).Scan(&connectionID, &scope)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume OAuth refresh token: %w", err)
	}
	pair := &models.OAuthTokenPair{
		AccessToken: accessToken, RefreshToken: refreshToken,
		ExpiresIn: int(time.Hour.Seconds()), Scope: scope,
	}
	if err := insertOAuthTokens(tx, connectionID, scope, pair); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit OAuth token refresh: %w", err)
	}
	return pair, nil
}

func AuthenticateOAuthAccessToken(
	database *sql.DB, raw, resource string,
) (*models.User, error) {
	if !validOAuthSecret(raw, oauthAccessTokenMarker) {
		return nil, nil
	}
	var user models.User
	var connectionID int64
	var alias sql.NullString
	err := database.QueryRow(`
		SELECT users.id, users.full_name, users.alias, authentication.email,
		       users.is_admin, users.is_contributor, users.created_at, users.updated_at, connection.id
		FROM oauth_access_tokens token
		JOIN oauth_connections connection ON connection.id = token.connection_id
		JOIN users ON users.id = connection.user_id
		JOIN local_authentications authentication ON authentication.user_id = users.id
		WHERE token.token_hash = $1 AND connection.resource = $2
		  AND token.expires_at > clock_timestamp()
	`, hashTokenSecret(raw), resource).Scan(
		&user.ID, &user.FullName, &alias, &user.Email,
		&user.IsAdmin, &user.IsContributor, &user.CreatedAt, &user.UpdatedAt, &connectionID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate OAuth access token: %w", err)
	}
	user.Alias = nullStringPointer(alias)
	_, _ = database.Exec(`
		UPDATE oauth_access_tokens
		SET last_used_at = clock_timestamp()
		WHERE token_hash = $1
		  AND (last_used_at IS NULL OR last_used_at < clock_timestamp() - INTERVAL '15 minutes')
	`, hashTokenSecret(raw))
	_, _ = database.Exec(`
		UPDATE oauth_connections
		SET last_used_at = clock_timestamp()
		WHERE id = $1
		  AND (last_used_at IS NULL OR last_used_at < clock_timestamp() - INTERVAL '15 minutes')
	`, connectionID)
	return &user, nil
}

// RevokeOAuthToken implements public-client token revocation without revealing
// whether the supplied secret existed.
func RevokeOAuthToken(database *sql.DB, raw, clientID string) error {
	var table string
	switch {
	case validOAuthSecret(raw, oauthAccessTokenMarker):
		table = "oauth_access_tokens"
	case validOAuthSecret(raw, oauthRefreshTokenMarker):
		table = "oauth_refresh_tokens"
	default:
		return nil
	}
	query := "DELETE FROM " + table + " token USING oauth_connections connection " +
		"WHERE token.connection_id = connection.id AND token.token_hash = $1 AND connection.client_id = $2"
	if _, err := database.Exec(query, hashTokenSecret(raw), clientID); err != nil {
		return fmt.Errorf("revoke OAuth token: %w", err)
	}
	return nil
}

func insertOAuthTokens(
	tx *sql.Tx, connectionID int64, scope string, pair *models.OAuthTokenPair,
) error {
	if _, err := tx.Exec(`
		INSERT INTO oauth_access_tokens (
			token_hash, connection_id, scope, expires_at
		)
		VALUES ($1, $2, $3, clock_timestamp() + INTERVAL '1 hour')
	`, hashTokenSecret(pair.AccessToken), connectionID, scope); err != nil {
		return fmt.Errorf("create OAuth access token: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO oauth_refresh_tokens (
			token_hash, connection_id, scope, expires_at
		)
		VALUES ($1, $2, $3, clock_timestamp() + INTERVAL '30 days')
	`, hashTokenSecret(pair.RefreshToken), connectionID, scope); err != nil {
		return fmt.Errorf("create OAuth refresh token: %w", err)
	}
	return nil
}

func newOAuthTokenPair() (string, string, error) {
	access, err := randomURLToken()
	if err != nil {
		return "", "", err
	}
	refresh, err := randomURLToken()
	if err != nil {
		return "", "", err
	}
	return oauthAccessTokenMarker + access, oauthRefreshTokenMarker + refresh, nil
}

func validOAuthSecret(raw, marker string) bool {
	if !strings.HasPrefix(raw, marker) {
		return false
	}
	secret := strings.TrimPrefix(raw, marker)
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

func cleanupExpiredOAuthTokens(q curriculumExecutor) {
	for _, table := range []string{
		"oauth_authorization_codes", "oauth_access_tokens", "oauth_refresh_tokens",
	} {
		_, _ = q.Exec("DELETE FROM " + table + " WHERE expires_at <= clock_timestamp()")
	}
	_, _ = q.Exec(`
		DELETE FROM oauth_connections connection
		WHERE NOT EXISTS (SELECT 1 FROM oauth_access_tokens WHERE connection_id = connection.id)
		  AND NOT EXISTS (SELECT 1 FROM oauth_refresh_tokens WHERE connection_id = connection.id)
	`)
}

func ListOAuthConnections(database *sql.DB, userID int64) ([]models.OAuthConnection, error) {
	cleanupExpiredOAuthTokens(database)
	rows, err := database.Query(`
		SELECT id, user_id, client_id, client_name, resource, authorized_at, last_used_at
		FROM oauth_connections
		WHERE user_id = $1
		ORDER BY authorized_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list OAuth connections: %w", err)
	}
	defer rows.Close()
	var connections []models.OAuthConnection
	for rows.Next() {
		var connection models.OAuthConnection
		var lastUsedAt sql.NullTime
		if err := rows.Scan(&connection.ID, &connection.UserID, &connection.ClientID,
			&connection.ClientName, &connection.Resource, &connection.AuthorizedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan OAuth connection: %w", err)
		}
		if lastUsedAt.Valid {
			connection.LastUsedAt = &lastUsedAt.Time
		}
		connections = append(connections, connection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate OAuth connections: %w", err)
	}
	return connections, nil
}

func DeleteOAuthConnection(database *sql.DB, userID, connectionID int64) (bool, error) {
	result, err := database.Exec(`DELETE FROM oauth_connections WHERE id = $1 AND user_id = $2`, connectionID, userID)
	if err != nil {
		return false, fmt.Errorf("delete OAuth connection: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
