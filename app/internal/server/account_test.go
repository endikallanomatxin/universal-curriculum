package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"universal-curriculum/internal/services"
)

func TestCreateAPITokenValidationRedirectsWithoutResubmission(t *testing.T) {
	server := &Server{}
	form := url.Values{"csrf_token": {"csrf"}, "name": {" "}}
	request := httptest.NewRequest(http.MethodPost, "/account/api-tokens", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = services.WithSession(request, 42, "csrf")
	response := httptest.NewRecorder()

	server.createAPIToken(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/account" {
		t.Fatalf("create API token response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	flash := server.takeAccountAPITokenFlash(42)
	if flash.Error == "" {
		t.Fatal("API token validation error was not preserved for the redirected page")
	}
	if repeated := server.takeAccountAPITokenFlash(42); repeated != (accountAPITokenFlash{}) {
		t.Fatalf("API token flash was available more than once: %#v", repeated)
	}
}

func TestCreateAPITokenRedirectPreservesNewSecretOnce(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, err := services.RegisterLocalUser(
		database, "Token Owner", "token-owner@example.com", "long-enough-password",
	)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Database: database}
	form := url.Values{"csrf_token": {"csrf"}, "name": {"Local CLI"}}
	request := httptest.NewRequest(http.MethodPost, "/account/api-tokens", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = services.WithSession(request, user.ID, "csrf")
	response := httptest.NewRecorder()

	server.createAPIToken(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/account" {
		t.Fatalf("create API token response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	flash := server.takeAccountAPITokenFlash(user.ID)
	if !strings.HasPrefix(flash.NewToken, "uc_api_") {
		t.Fatalf("redirected API token secret = %q", flash.NewToken)
	}
	if repeated := server.takeAccountAPITokenFlash(user.ID); repeated != (accountAPITokenFlash{}) {
		t.Fatalf("API token secret was available more than once: %#v", repeated)
	}
}
