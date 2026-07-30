package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"

	"golang.org/x/crypto/bcrypt"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

func RequestPasswordReset(
	ctx context.Context,
	database *sql.DB,
	sender EmailSender,
	appBaseURL, email string,
) error {
	email = models.NormalizeEmail(email)
	userID, exists, err := db.GetLocalUserIDByEmail(database, email)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	token, err := db.CreatePasswordResetToken(database, userID)
	if errors.Is(err, db.ErrPasswordResetRecentlyAsked) {
		return nil
	}
	if err != nil {
		return err
	}
	resetURL := PasswordResetURL(appBaseURL, token.Token)
	return sender.Send(ctx, passwordResetEmail(email, resetURL, token.Token))
}

func ResetPassword(database *sql.DB, token, password string) error {
	if err := models.ValidatePassword(password); err != nil {
		return err
	}
	valid, err := db.PasswordResetTokenIsValid(database, token)
	if err != nil {
		return err
	}
	if !valid {
		return db.ErrInvalidPasswordResetToken
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash reset password: %w", err)
	}
	return db.ResetPasswordWithToken(database, token, passwordHash)
}

func PasswordResetURL(appBaseURL, token string) string {
	target, err := url.Parse(appBaseURL)
	if err != nil {
		return ""
	}
	target.Path = "/auth/reset-password"
	target.RawPath = ""
	target.RawQuery = url.Values{"token": {token}}.Encode()
	target.Fragment = ""
	return target.String()
}

func passwordResetEmail(recipient, resetURL, token string) EmailMessage {
	escapedURL := html.EscapeString(resetURL)
	return EmailMessage{
		To:      recipient,
		Subject: "Reset your Universal Curriculum password",
		Text: fmt.Sprintf(
			"We received a request to reset your Universal Curriculum password.\n\n"+
				"Choose a new password:\n%s\n\n"+
				"This link expires in one hour. If you did not request this change, you can ignore this email.",
			resetURL,
		),
		HTML: fmt.Sprintf(
			"<p>We received a request to reset your Universal Curriculum password.</p>"+
				"<p><a href=\"%s\">Choose a new password</a></p>"+
				"<p>This link expires in one hour. If you did not request this change, you can ignore this email.</p>",
			escapedURL,
		),
		IdempotencyKey: emailIdempotencyKey("password-reset", token),
	}
}

func emailIdempotencyKey(kind, token string) string {
	hash := sha256.Sum256([]byte(token))
	return kind + "/" + hex.EncodeToString(hash[:])
}
