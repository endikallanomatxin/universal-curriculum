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
			code_hash, user_id, client_id, redirect_uri, resource, scope,
			code_challenge, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, clock_timestamp() + INTERVAL '5 minutes')
	`, hashTokenSecret(raw), grant.UserID, grant.ClientID, grant.RedirectURI,
		grant.Resource, grant.Scope, grant.CodeChallenge); err != nil {
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
		RETURNING user_id, client_id, redirect_uri, resource, scope, code_challenge
	`, hashTokenSecret(rawCode), clientID, redirectURI, resource, codeChallenge).Scan(
		&grant.UserID, &grant.ClientID, &grant.RedirectURI, &grant.Resource,
		&grant.Scope, &grant.CodeChallenge,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume OAuth authorization code: %w", err)
	}
	cleanupExpiredOAuthTokens(tx)
	pair := &models.OAuthTokenPair{
		AccessToken: accessToken, RefreshToken: refreshToken,
		ExpiresIn: int(time.Hour.Seconds()), Scope: grant.Scope,
	}
	if err := insertOAuthTokens(tx, grant.UserID, grant.ClientID, grant.Resource, grant.Scope, pair); err != nil {
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
	var userID int64
	var scope string
	err = tx.QueryRow(`
		DELETE FROM oauth_refresh_tokens
		WHERE token_hash = $1 AND client_id = $2 AND resource = $3
		  AND expires_at > clock_timestamp()
		RETURNING user_id, scope
	`, hashTokenSecret(rawRefreshToken), clientID, resource).Scan(&userID, &scope)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume OAuth refresh token: %w", err)
	}
	cleanupExpiredOAuthTokens(tx)
	pair := &models.OAuthTokenPair{
		AccessToken: accessToken, RefreshToken: refreshToken,
		ExpiresIn: int(time.Hour.Seconds()), Scope: scope,
	}
	if err := insertOAuthTokens(tx, userID, clientID, resource, scope, pair); err != nil {
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
	var alias sql.NullString
	err := database.QueryRow(`
		SELECT users.id, users.full_name, users.alias, authentication.email,
		       users.is_admin, users.created_at, users.updated_at
		FROM oauth_access_tokens token
		JOIN users ON users.id = token.user_id
		JOIN local_authentications authentication ON authentication.user_id = users.id
		WHERE token.token_hash = $1 AND token.resource = $2
		  AND token.expires_at > clock_timestamp()
	`, hashTokenSecret(raw), resource).Scan(
		&user.ID, &user.FullName, &alias, &user.Email,
		&user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
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
	query := "DELETE FROM " + table + " WHERE token_hash = $1 AND client_id = $2"
	if _, err := database.Exec(query, hashTokenSecret(raw), clientID); err != nil {
		return fmt.Errorf("revoke OAuth token: %w", err)
	}
	return nil
}

func insertOAuthTokens(
	tx *sql.Tx, userID int64, clientID, resource, scope string, pair *models.OAuthTokenPair,
) error {
	if _, err := tx.Exec(`
		INSERT INTO oauth_access_tokens (
			token_hash, user_id, client_id, resource, scope, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp() + INTERVAL '1 hour')
	`, hashTokenSecret(pair.AccessToken), userID, clientID, resource, scope); err != nil {
		return fmt.Errorf("create OAuth access token: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO oauth_refresh_tokens (
			token_hash, user_id, client_id, resource, scope, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp() + INTERVAL '30 days')
	`, hashTokenSecret(pair.RefreshToken), userID, clientID, resource, scope); err != nil {
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
}
