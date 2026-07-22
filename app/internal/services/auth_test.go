package services

import (
	"net/http/httptest"
	"testing"
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
