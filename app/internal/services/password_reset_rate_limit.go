package services

import (
	"database/sql"
	"fmt"
	"time"

	"universal-curriculum/internal/db"
)

const (
	passwordResetRequestBlockThreshold = 6
	passwordResetAttemptBlockThreshold = 11
	passwordResetRateWindow            = 15 * time.Minute
	passwordResetBlockDuration         = 15 * time.Minute
	passwordResetRateRetention         = 24 * time.Hour
)

type PasswordResetRateLimiter struct {
	database *sql.DB
	now      func() time.Time
}

func NewPasswordResetRateLimiter(database *sql.DB) *PasswordResetRateLimiter {
	return &PasswordResetRateLimiter{database: database, now: time.Now}
}

// RegisterRequest records a recovery-email request. The sixth request from the
// same IP within the window is rejected.
func (limiter *PasswordResetRateLimiter) RegisterRequest(ip string) (bool, error) {
	return limiter.register(ip, db.AuthenticationRateScopePasswordResetRequest, passwordResetRequestBlockThreshold)
}

// RegisterAttempt records a validly formed password-reset submission. The
// eleventh attempt from the same IP within the window is rejected.
func (limiter *PasswordResetRateLimiter) RegisterAttempt(ip string) (bool, error) {
	return limiter.register(ip, db.AuthenticationRateScopePasswordResetAttempt, passwordResetAttemptBlockThreshold)
}

func (limiter *PasswordResetRateLimiter) register(ip, scope string, threshold int) (bool, error) {
	if limiter == nil || limiter.database == nil || ip == "" {
		return false, fmt.Errorf("invalid password reset rate limit input")
	}
	tx, err := limiter.database.Begin()
	if err != nil {
		return false, fmt.Errorf("begin password reset rate limit update: %w", err)
	}
	defer tx.Rollback()

	now := limiter.now()
	blocked, err := db.RecordAuthenticationRateEvent(
		tx,
		scope,
		ip,
		threshold,
		now.Add(-passwordResetRateWindow),
		now.Add(passwordResetBlockDuration),
		now,
	)
	if err != nil {
		return false, err
	}
	if err := db.DeleteOldAuthenticationRateLimits(tx, now.Add(-passwordResetRateRetention), now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit password reset rate limit update: %w", err)
	}
	return blocked, nil
}
