package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"universal-curriculum/internal/models"
)

const (
	PasswordResetTokenLifetime   = time.Hour
	PasswordResetRequestCooldown = 5 * time.Minute
)

var (
	ErrInvalidPasswordResetToken  = errors.New("invalid or expired password reset token")
	ErrPasswordResetRecentlyAsked = errors.New("password reset recently requested")
)

func CreatePasswordResetToken(database *sql.DB, userID int64) (*models.PasswordResetToken, error) {
	token, err := randomURLToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiresAt := now.Add(PasswordResetTokenLifetime)
	result, err := database.Exec(`
		INSERT INTO password_reset_tokens (token_hash, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET token_hash = EXCLUDED.token_hash,
		    expires_at = EXCLUDED.expires_at,
		    created_at = EXCLUDED.created_at
		WHERE password_reset_tokens.created_at <= $5
	`, passwordResetTokenHash(token), userID, expiresAt, now, now.Add(-PasswordResetRequestCooldown))
	if err != nil {
		return nil, fmt.Errorf("store password reset token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read password reset token result: %w", err)
	}
	if rowsAffected == 0 {
		return nil, ErrPasswordResetRecentlyAsked
	}
	return &models.PasswordResetToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

func PasswordResetTokenIsValid(database *sql.DB, token string) (bool, error) {
	if !validPasswordResetToken(token) {
		return false, nil
	}
	var valid bool
	err := database.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM password_reset_tokens
			WHERE token_hash = $1
			  AND expires_at > NOW()
		)
	`, passwordResetTokenHash(token)).Scan(&valid)
	if err != nil {
		return false, fmt.Errorf("validate password reset token: %w", err)
	}
	return valid, nil
}

func ResetPasswordWithToken(database *sql.DB, token string, passwordHash []byte) (err error) {
	if !validPasswordResetToken(token) {
		return ErrInvalidPasswordResetToken
	}
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin password reset: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userID int64
	err = tx.QueryRow(`
		SELECT user_id
		FROM password_reset_tokens
		WHERE token_hash = $1
		  AND expires_at > NOW()
		FOR UPDATE
	`, passwordResetTokenHash(token)).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidPasswordResetToken
	}
	if err != nil {
		return fmt.Errorf("consume password reset token: %w", err)
	}
	if _, err = tx.Exec(`
		UPDATE local_authentications
		SET password_hash = $2, updated_at = NOW()
		WHERE user_id = $1
	`, userID, passwordHash); err != nil {
		return fmt.Errorf("update local password: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("revoke sessions after password reset: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM password_reset_tokens WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete password reset token: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit password reset: %w", err)
	}
	return nil
}

func validPasswordResetToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func passwordResetTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
