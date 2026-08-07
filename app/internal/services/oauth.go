package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

var ErrInvalidOAuthGrant = errors.New("invalid or expired OAuth grant")

func ExchangeOAuthAuthorizationCode(
	database *sql.DB, code, clientID, redirectURI, resource, verifier string,
) (*models.OAuthTokenPair, error) {
	if !validPKCEVerifier(verifier) {
		return nil, ErrInvalidOAuthGrant
	}
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	pair, err := db.ExchangeOAuthAuthorizationCode(
		database, code, clientID, redirectURI, resource, challenge,
	)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, ErrInvalidOAuthGrant
	}
	return pair, nil
}

func validPKCEVerifier(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '.' || character == '_' || character == '~' {
			continue
		}
		return false
	}
	return true
}

func RefreshOAuthAccessToken(
	database *sql.DB, refreshToken, clientID, resource string,
) (*models.OAuthTokenPair, error) {
	pair, err := db.RefreshOAuthAccessToken(database, refreshToken, clientID, resource)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, ErrInvalidOAuthGrant
	}
	return pair, nil
}
