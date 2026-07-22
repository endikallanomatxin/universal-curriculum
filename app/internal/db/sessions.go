package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	SessionIdleTimeout            = 30 * 24 * time.Hour
	SessionActivityUpdateInterval = 24 * time.Hour
	SessionRotationInterval       = 7 * 24 * time.Hour
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

type SessionUse struct {
	UserID        int64
	CSRFToken     string
	RotatedToken  string
	RefreshCookie bool
}

func CreateSession(database *sql.DB, userID int64) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	_, err = database.Exec(`
		INSERT INTO sessions
		    (token_hash, user_id, csrf_token, expires_at, last_activity_at, rotated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, hashSessionToken(token), userID, csrfToken, now.Add(SessionIdleTimeout), now)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

func UseSession(database *sql.DB, token string) (result SessionUse, err error) {
	tx, err := database.Begin()
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now()
	oldHash := hashSessionToken(token)
	var expiresAt, lastActivityAt, rotatedAt time.Time
	err = tx.QueryRow(`
		SELECT user_id, csrf_token, expires_at, last_activity_at, rotated_at
		FROM sessions
		WHERE token_hash = $1
		FOR UPDATE
	`, oldHash).Scan(&result.UserID, &result.CSRFToken, &expiresAt, &lastActivityAt, &rotatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionUse{}, ErrSessionNotFound
	}
	if err != nil {
		return result, fmt.Errorf("get session: %w", err)
	}
	if !now.Before(expiresAt) || now.Sub(lastActivityAt) >= SessionIdleTimeout {
		if _, err = tx.Exec(`DELETE FROM sessions WHERE token_hash = $1`, oldHash); err != nil {
			return result, fmt.Errorf("delete expired session: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return result, err
		}
		return SessionUse{}, ErrSessionExpired
	}

	if now.Sub(rotatedAt) >= SessionRotationInterval {
		result.RotatedToken, err = randomToken()
		if err != nil {
			return result, err
		}
		_, err = tx.Exec(`
			UPDATE sessions
			SET token_hash = $2, expires_at = $3, last_activity_at = $4, rotated_at = $4
			WHERE token_hash = $1
		`, oldHash, hashSessionToken(result.RotatedToken), now.Add(SessionIdleTimeout), now)
		result.RefreshCookie = true
	} else if now.Sub(lastActivityAt) >= SessionActivityUpdateInterval {
		_, err = tx.Exec(`
			UPDATE sessions SET expires_at = $2, last_activity_at = $3 WHERE token_hash = $1
		`, oldHash, now.Add(SessionIdleTimeout), now)
		result.RefreshCookie = true
	}
	if err != nil {
		return result, fmt.Errorf("refresh session: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return result, fmt.Errorf("commit session use: %w", err)
	}
	return result, nil
}

func DeleteSession(database *sql.DB, token string) error {
	if _, err := database.Exec(`DELETE FROM sessions WHERE token_hash = $1`, hashSessionToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
