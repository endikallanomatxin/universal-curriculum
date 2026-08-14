package services

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"net/url"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

const ContributorInvitationLifetime = 7 * 24 * time.Hour

func InviteContributor(ctx context.Context, database *sql.DB, sender EmailSender, appBaseURL, email string, invitedBy int64) (*models.ContributorInvitation, error) {
	email = models.NormalizeEmail(email)
	if err := models.ValidateEmail(email); err != nil {
		return nil, err
	}
	invitation, err := db.CreateContributorInvitation(database, email, invitedBy, time.Now().Add(ContributorInvitationLifetime))
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(appBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse application URL: %w", err)
	}
	target.Path = "/auth/contributor-invitation"
	target.RawQuery = url.Values{"token": {invitation.Token}}.Encode()
	invitationURL := target.String()
	err = sender.Send(ctx, EmailMessage{
		To: email, Subject: "Invitation to contribute to Universal Curriculum",
		Text:           fmt.Sprintf("You have been invited to contribute to Universal Curriculum.\n\nAccept the invitation:\n%s\n\nThis link expires in seven days.", invitationURL),
		HTML:           fmt.Sprintf("<p>You have been invited to contribute to Universal Curriculum.</p><p><a href=\"%s\">Accept the invitation</a></p><p>This link expires in seven days.</p>", html.EscapeString(invitationURL)),
		IdempotencyKey: emailIdempotencyKey("contributor-invitation", invitation.Token),
	})
	if err != nil {
		return nil, err
	}
	return invitation, nil
}
