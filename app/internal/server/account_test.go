package server

import (
	"fmt"
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

func TestRevokeOAuthConnectionDeletesOwnedConnection(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, err := services.RegisterLocalUser(
		database, "Connection Owner", "connection-owner@example.com", "long-enough-password",
	)
	if err != nil {
		t.Fatal(err)
	}
	var connectionID int64
	if err := database.QueryRow(`
		INSERT INTO oauth_connections (user_id, client_id, client_name, resource)
		VALUES ($1, 'https://client.example/metadata.json', 'Example client', 'https://curriculum.example/mcp')
		RETURNING id
	`, user.ID).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	server := &Server{Database: database}
	form := url.Values{"csrf_token": {"csrf"}}
	request := httptest.NewRequest(http.MethodPost, "/account/oauth-connections/1/revoke", strings.NewReader(form.Encode()))
	request.SetPathValue("id", fmt.Sprint(connectionID))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = services.WithSession(request, user.ID, "csrf")
	response := httptest.NewRecorder()

	server.revokeOAuthConnection(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/account" {
		t.Fatalf("revoke connection response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	var remaining int
	if err := database.QueryRow(`SELECT count(*) FROM oauth_connections WHERE id = $1`, connectionID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatal("OAuth connection was not deleted")
	}
}
