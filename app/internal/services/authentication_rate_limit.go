package services

import (
	"database/sql"
	"fmt"
	"time"

	"universal-curriculum/internal/db"
)

const (
	loginBlockThreshold                = 11
	registrationBlockThreshold         = 6
	passwordResetRequestBlockThreshold = 6
	passwordResetAttemptBlockThreshold = 11
	authenticationRateWindow           = 15 * time.Minute
	authenticationBlockDuration        = 15 * time.Minute
	authenticationRateRetention        = 24 * time.Hour
)

type AuthenticationRateLimiter struct {
	database *sql.DB
	now      func() time.Time
}

func NewAuthenticationRateLimiter(database *sql.DB) *AuthenticationRateLimiter {
	return &AuthenticationRateLimiter{database: database, now: time.Now}
}

// RegisterLogin records a login submission. The eleventh submission from the
// same IP within the window is rejected, before password hashing is attempted.
func (limiter *AuthenticationRateLimiter) RegisterLogin(ip string) (bool, error) {
	return limiter.register(ip, db.AuthenticationRateScopeLogin, loginBlockThreshold)
}

// RegisterRegistration records a registration submission. The sixth
// submission from the same IP within the window is rejected.
func (limiter *AuthenticationRateLimiter) RegisterRegistration(ip string) (bool, error) {
	return limiter.register(ip, db.AuthenticationRateScopeRegistration, registrationBlockThreshold)
}

// RegisterPasswordResetRequest records a recovery-email request. The sixth
// request from the same IP within the window is rejected.
func (limiter *AuthenticationRateLimiter) RegisterPasswordResetRequest(ip string) (bool, error) {
	return limiter.register(ip, db.AuthenticationRateScopePasswordResetRequest, passwordResetRequestBlockThreshold)
}

// RegisterPasswordResetAttempt records a validly formed password-reset
// submission. The eleventh attempt from the same IP within the window is rejected.
func (limiter *AuthenticationRateLimiter) RegisterPasswordResetAttempt(ip string) (bool, error) {
	return limiter.register(ip, db.AuthenticationRateScopePasswordResetAttempt, passwordResetAttemptBlockThreshold)
}

func (limiter *AuthenticationRateLimiter) register(ip, scope string, threshold int) (bool, error) {
	if limiter == nil || limiter.database == nil || ip == "" {
		return false, fmt.Errorf("invalid authentication rate limit input")
	}
	tx, err := limiter.database.Begin()
	if err != nil {
		return false, fmt.Errorf("begin authentication rate limit update: %w", err)
	}
	defer tx.Rollback()

	now := limiter.now()
	blocked, err := db.RecordAuthenticationRateEvent(
		tx,
		scope,
		ip,
		threshold,
		now.Add(-authenticationRateWindow),
		now.Add(authenticationBlockDuration),
		now,
	)
	if err != nil {
		return false, err
	}
	if err := db.DeleteOldAuthenticationRateLimits(tx, now.Add(-authenticationRateRetention), now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit authentication rate limit update: %w", err)
	}
	return blocked, nil
}
