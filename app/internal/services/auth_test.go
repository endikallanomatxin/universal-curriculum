package services

import (
	"errors"
	"net/http/httptest"
	"testing"

	"universal-curriculum/internal/models"
)

func TestSessionContext(t *testing.T) {
	request := WithSession(httptest.NewRequest("GET", "/", nil), 42, "csrf")
	if userID, ok := SessionUserID(request); !ok || userID != 42 {
		t.Fatalf("SessionUserID() = %d, %v", userID, ok)
	}
	if !ValidCSRFToken(request, "csrf") || ValidCSRFToken(request, "wrong") {
		t.Fatal("CSRF token validation returned an unexpected result")
	}
}

func TestRegisterLocalUserValidatesBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name     string
		fullName string
		email    string
		password string
		wantErr  error
	}{
		{name: "name", email: "student@example.com", password: "long-enough-password", wantErr: models.ErrFullNameRequired},
		{name: "email", fullName: "Student", email: "invalid", password: "long-enough-password", wantErr: models.ErrInvalidEmail},
		{name: "password", fullName: "Student", email: "student@example.com", password: "short", wantErr: models.ErrPasswordTooShort},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := RegisterLocalUser(nil, test.fullName, test.email, test.password); !errors.Is(err, test.wantErr) {
				t.Fatalf("RegisterLocalUser() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
