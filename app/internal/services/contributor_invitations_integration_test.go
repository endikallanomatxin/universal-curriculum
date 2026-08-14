package services

import (
	"errors"
	"testing"
	"time"

	"universal-curriculum/internal/db"
)

func TestContributorInvitationMatchesEmailAndCanOnlyBeAcceptedOnce(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "contributor_invitation")
	administrator, _ := db.CreateLocalUser(database, "Administrator", "admin@example.com", []byte("hash"))
	invited, _ := db.CreateLocalUser(database, "Invited", "invited@example.com", []byte("hash"))
	other, _ := db.CreateLocalUser(database, "Other", "other@example.com", []byte("hash"))
	invitation, err := db.CreateContributorInvitation(database, "INVITED@example.com", administrator.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AcceptContributorInvitation(database, invitation.Token, other.ID); !errors.Is(err, db.ErrInvalidContributorInvitation) {
		t.Fatalf("other email acceptance = %v", err)
	}
	if err := db.AcceptContributorInvitation(database, invitation.Token, invited.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.AcceptContributorInvitation(database, invitation.Token, invited.ID); !errors.Is(err, db.ErrInvalidContributorInvitation) {
		t.Fatalf("second acceptance = %v", err)
	}
	stored, err := db.GetUserByID(database, invited.ID)
	if err != nil || !stored.IsContributor {
		t.Fatalf("invited user = %#v, err %v", stored, err)
	}
}

func TestRevokedContributorInvitationCannotBeAccepted(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "revoked_contributor_invitation")
	administrator, _ := db.CreateLocalUser(database, "Administrator", "admin@example.com", []byte("hash"))
	invited, _ := db.CreateLocalUser(database, "Invited", "invited@example.com", []byte("hash"))
	invitation, err := db.CreateContributorInvitation(database, invited.Email, administrator.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.RevokeContributorInvitation(database, invitation.ID); err != nil || !revoked {
		t.Fatalf("revoke = %v, %v", revoked, err)
	}
	if err := db.AcceptContributorInvitation(database, invitation.Token, invited.ID); !errors.Is(err, db.ErrInvalidContributorInvitation) {
		t.Fatalf("revoked acceptance = %v", err)
	}
}
