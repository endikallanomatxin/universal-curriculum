package db

import (
	"fmt"
	"time"
)

const (
	AuthenticationRateScopeLogin                = "login_ip"
	AuthenticationRateScopeRegistration         = "registration_ip"
	AuthenticationRateScopePasswordResetRequest = "password_reset_request_ip"
	AuthenticationRateScopePasswordResetAttempt = "password_reset_attempt_ip"
)

func RecordAuthenticationRateEvent(
	query QueryExecutor,
	scope, key string,
	limit int,
	windowCutoff, blockUntil, now time.Time,
) (bool, error) {
	var blockedUntil *time.Time
	err := query.QueryRow(`
		INSERT INTO authentication_rate_limits (
			scope, key, window_started_at, event_count, blocked_until, updated_at
		) VALUES ($1, $2, $3, 1, NULL, $3)
		ON CONFLICT (scope, key) DO UPDATE SET
			window_started_at = CASE
				WHEN authentication_rate_limits.window_started_at <= $5 THEN $3
				ELSE authentication_rate_limits.window_started_at
			END,
			event_count = CASE
				WHEN authentication_rate_limits.window_started_at <= $5 THEN 1
				ELSE authentication_rate_limits.event_count + 1
			END,
			blocked_until = CASE
				WHEN authentication_rate_limits.blocked_until > $3 THEN authentication_rate_limits.blocked_until
				WHEN (CASE
					WHEN authentication_rate_limits.window_started_at <= $5 THEN 1
					ELSE authentication_rate_limits.event_count + 1
				END) >= $4 THEN $6
				ELSE NULL
			END,
			updated_at = $3
		RETURNING blocked_until
	`, scope, key, now, limit, windowCutoff, blockUntil).Scan(&blockedUntil)
	if err != nil {
		return false, fmt.Errorf("record authentication rate event: %w", err)
	}
	return blockedUntil != nil && blockedUntil.After(now), nil
}

func DeleteOldAuthenticationRateLimits(query QueryExecutor, cutoff, now time.Time) error {
	if _, err := query.Exec(`
		DELETE FROM authentication_rate_limits
		WHERE updated_at < $1 AND (blocked_until IS NULL OR blocked_until <= $2)
	`, cutoff, now); err != nil {
		return fmt.Errorf("delete old authentication rate limits: %w", err)
	}
	return nil
}
