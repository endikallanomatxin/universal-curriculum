package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"universal-curriculum/internal/models"
)

var ErrInvalidContributorInvitation = errors.New("invalid or expired contributor invitation")

func CreateContributorInvitation(database *sql.DB, email string, invitedBy int64, expiresAt time.Time) (*models.ContributorInvitation, error) {
	token, err := randomURLToken()
	if err != nil {
		return nil, err
	}
	invitation := &models.ContributorInvitation{Email: models.NormalizeEmail(email), Token: token, InvitedBy: invitedBy, ExpiresAt: expiresAt}
	err = database.QueryRow(`
		INSERT INTO contributor_invitations (email, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, invitation.Email, contributorInvitationTokenHash(token), invitedBy, expiresAt).Scan(&invitation.ID, &invitation.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create contributor invitation: %w", err)
	}
	return invitation, nil
}

func ListContributorInvitations(database *sql.DB) ([]models.ContributorInvitation, error) {
	rows, err := database.Query(`
		SELECT id, email, invited_by, accepted_by, expires_at, accepted_at, revoked_at, created_at
		FROM contributor_invitations
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list contributor invitations: %w", err)
	}
	defer rows.Close()
	var invitations []models.ContributorInvitation
	for rows.Next() {
		var invitation models.ContributorInvitation
		var acceptedBy sql.NullInt64
		var acceptedAt, revokedAt sql.NullTime
		if err := rows.Scan(&invitation.ID, &invitation.Email, &invitation.InvitedBy, &acceptedBy, &invitation.ExpiresAt, &acceptedAt, &revokedAt, &invitation.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contributor invitation: %w", err)
		}
		if acceptedBy.Valid {
			invitation.AcceptedBy = &acceptedBy.Int64
		}
		if acceptedAt.Valid {
			invitation.AcceptedAt = &acceptedAt.Time
		}
		if revokedAt.Valid {
			invitation.RevokedAt = &revokedAt.Time
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func ValidContributorInvitationEmail(database *sql.DB, token string) (string, error) {
	if token == "" {
		return "", ErrInvalidContributorInvitation
	}
	var email string
	err := database.QueryRow(`SELECT email FROM contributor_invitations WHERE token_hash = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > clock_timestamp()`, contributorInvitationTokenHash(token)).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalidContributorInvitation
	}
	if err != nil {
		return "", fmt.Errorf("validate contributor invitation: %w", err)
	}
	return email, nil
}

func AcceptContributorInvitation(database *sql.DB, token string, userID int64) error {
	if token == "" {
		return ErrInvalidContributorInvitation
	}
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin contributor invitation acceptance: %w", err)
	}
	defer tx.Rollback()
	if err := AcceptContributorInvitationInTransaction(tx, token, userID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit contributor invitation acceptance: %w", err)
	}
	return nil
}

func AcceptContributorInvitationInTransaction(tx *sql.Tx, token string, userID int64) error {
	if token == "" {
		return ErrInvalidContributorInvitation
	}
	result, err := tx.Exec(`
		UPDATE contributor_invitations invitation
		SET accepted_by = $2, accepted_at = clock_timestamp()
		FROM local_authentications authentication
		WHERE invitation.token_hash = $1
		  AND invitation.accepted_at IS NULL AND invitation.revoked_at IS NULL
		  AND invitation.expires_at > clock_timestamp()
		  AND authentication.user_id = $2 AND authentication.email = invitation.email
	`, contributorInvitationTokenHash(token), userID)
	if err != nil {
		return fmt.Errorf("accept contributor invitation: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read contributor invitation result: %w", err)
	}
	if count != 1 {
		return ErrInvalidContributorInvitation
	}
	if _, err := tx.Exec(`UPDATE users SET is_contributor = TRUE, updated_at = clock_timestamp() WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("grant contributor access: %w", err)
	}
	return nil
}

func RevokeContributorInvitation(database *sql.DB, invitationID int64) (bool, error) {
	result, err := database.Exec(`UPDATE contributor_invitations SET revoked_at = clock_timestamp() WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL`, invitationID)
	if err != nil {
		return false, fmt.Errorf("revoke contributor invitation: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func contributorInvitationTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
