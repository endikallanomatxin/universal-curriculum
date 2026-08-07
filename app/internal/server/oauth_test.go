package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/services"
)

type staticOAuthClientResolver struct {
	metadata *OAuthClientMetadata
	err      error
}

func (resolver staticOAuthClientResolver) Resolve(context.Context, string) (*OAuthClientMetadata, error) {
	return resolver.metadata, resolver.err
}

func TestOAuthDiscoveryMetadata(t *testing.T) {
	server := &Server{Config: Config{AppBaseURL: "https://curriculum.example"}}

	resourceResponse := httptest.NewRecorder()
	server.oauthProtectedResourceMetadata(resourceResponse, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))
	if resourceResponse.Code != http.StatusOK || resourceResponse.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("protected resource response = %d, headers %#v", resourceResponse.Code, resourceResponse.Header())
	}
	var resource map[string]any
	if err := json.Unmarshal(resourceResponse.Body.Bytes(), &resource); err != nil {
		t.Fatal(err)
	}
	if resource["resource"] != "https://curriculum.example/mcp" {
		t.Fatalf("resource metadata = %#v", resource)
	}

	serverResponse := httptest.NewRecorder()
	server.oauthAuthorizationServerMetadata(serverResponse, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	var metadata map[string]any
	if err := json.Unmarshal(serverResponse.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["issuer"] != "https://curriculum.example" || metadata["client_id_metadata_document_supported"] != true {
		t.Fatalf("authorization server metadata = %#v", metadata)
	}
}

func TestOAuthAuthorizationRequiresConsentAndPreservesState(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })

	clientID := "https://chat.example/oauth-client.json"
	redirectURI := "https://chat.example/oauth/callback"
	server := &Server{
		Config: Config{AppBaseURL: "https://curriculum.example"},
		OAuthClients: staticOAuthClientResolver{metadata: &OAuthClientMetadata{
			ClientID: clientID, ClientName: "Example agent", RedirectURIs: []string{redirectURI},
			TokenEndpointAuthMethod: "none",
		}},
	}
	templates, err := services.LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	server.Templates = templates
	values := validOAuthAuthorizationValues(clientID, redirectURI)

	getRequest := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+values.Encode(), nil)
	getRequest = services.WithSession(getRequest, 42, "csrf-value")
	getResponse := httptest.NewRecorder()
	server.oauthAuthorize(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), "Authorize Example agent") {
		t.Fatalf("consent response = %d: %s", getResponse.Code, getResponse.Body.String())
	}

	values.Set("csrf_token", "csrf-value")
	values.Set("decision", "deny")
	postRequest := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(values.Encode()))
	postRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRequest = services.WithSession(postRequest, 42, "csrf-value")
	postResponse := httptest.NewRecorder()
	server.oauthAuthorize(postResponse, postRequest)
	if postResponse.Code != http.StatusFound {
		t.Fatalf("denial response = %d: %s", postResponse.Code, postResponse.Body.String())
	}
	location, err := url.Parse(postResponse.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("error") != "access_denied" || location.Query().Get("state") != "opaque-state" {
		t.Fatalf("denial redirect = %s", location)
	}
}

func TestOAuthAuthorizationRejectsUnregisteredRedirect(t *testing.T) {
	clientID := "https://chat.example/oauth-client.json"
	server := &Server{
		Config: Config{AppBaseURL: "https://curriculum.example"},
		OAuthClients: staticOAuthClientResolver{metadata: &OAuthClientMetadata{
			ClientID: clientID, ClientName: "Example agent",
			RedirectURIs: []string{"https://chat.example/oauth/callback"},
		}},
	}
	values := validOAuthAuthorizationValues(clientID, "https://attacker.example/callback")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+values.Encode(), nil)
	request = services.WithSession(request, 42, "csrf-value")
	server.oauthAuthorize(response, request)
	if response.Code != http.StatusBadRequest || response.Header().Get("Location") != "" {
		t.Fatalf("response = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
}

func TestOAuthAuthorizationRequiresValidS256Challenge(t *testing.T) {
	clientID := "https://chat.example/oauth-client.json"
	redirectURI := "https://chat.example/oauth/callback"
	server := &Server{
		Config: Config{AppBaseURL: "https://curriculum.example"},
		OAuthClients: staticOAuthClientResolver{metadata: &OAuthClientMetadata{
			ClientID: clientID, ClientName: "Example agent", RedirectURIs: []string{redirectURI},
		}},
	}
	values := validOAuthAuthorizationValues(clientID, redirectURI)
	values.Set("code_challenge", strings.Repeat("!", 43))
	request := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+values.Encode(), nil)
	request = services.WithSession(request, 42, "csrf-value")
	response := httptest.NewRecorder()
	server.oauthAuthorize(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
}

func TestOAuthAuthorizationCodeFlowWithPostgreSQL(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, err := services.RegisterLocalUser(database, "OAuth integration", "oauth-flow@example.com", "long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	clientID := "https://chat.example/oauth-client.json"
	redirectURI := "https://chat.example/oauth/callback"
	verifier := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	server := &Server{
		Config:   Config{AppBaseURL: "https://curriculum.example"},
		Database: database,
		OAuthClients: staticOAuthClientResolver{metadata: &OAuthClientMetadata{
			ClientID: clientID, ClientName: "Example agent", RedirectURIs: []string{redirectURI},
			TokenEndpointAuthMethod: "none",
		}},
	}
	values := validOAuthAuthorizationValues(clientID, redirectURI)
	values.Set("code_challenge", challenge)
	values.Set("csrf_token", "csrf-value")
	values.Set("decision", "allow")
	request := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = services.WithSession(request, user.ID, "csrf-value")
	response := httptest.NewRecorder()
	server.oauthAuthorize(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("authorization response = %d: %s", response.Code, response.Body.String())
	}
	redirect, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	code := redirect.Query().Get("code")
	if code == "" || redirect.Query().Get("iss") != "https://curriculum.example" {
		t.Fatalf("authorization redirect = %s", redirect)
	}

	tokenValues := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"resource":      {"https://curriculum.example/mcp"},
		"code":          {code},
		"code_verifier": {verifier},
	}
	tokenRequest := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenValues.Encode()))
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResponse := httptest.NewRecorder()
	server.oauthToken(tokenResponse, tokenRequest)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("token response = %d: %s", tokenResponse.Code, tokenResponse.Body.String())
	}
	var tokenBody struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &tokenBody); err != nil {
		t.Fatal(err)
	}
	if tokenBody.AccessToken == "" || tokenBody.RefreshToken == "" || tokenBody.TokenType != "Bearer" || tokenBody.ExpiresIn != 3600 {
		t.Fatalf("token body = %#v", tokenBody)
	}

	// Authorization codes are one-time secrets.
	reuseRequest := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenValues.Encode()))
	reuseRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reuseResponse := httptest.NewRecorder()
	server.oauthToken(reuseResponse, reuseRequest)
	if reuseResponse.Code != http.StatusBadRequest || !strings.Contains(reuseResponse.Body.String(), `"invalid_grant"`) {
		t.Fatalf("code reuse response = %d: %s", reuseResponse.Code, reuseResponse.Body.String())
	}

	revokeValues := url.Values{"client_id": {clientID}, "token": {tokenBody.AccessToken}}
	revokeRequest := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(revokeValues.Encode()))
	revokeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revokeResponse := httptest.NewRecorder()
	server.oauthRevoke(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("revocation response = %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	authenticated, err := db.AuthenticateOAuthAccessToken(database, tokenBody.AccessToken, "https://curriculum.example/mcp")
	if err != nil || authenticated != nil {
		t.Fatalf("revoked token authentication = %#v, %v", authenticated, err)
	}
}

func validOAuthAuthorizationValues(clientID, redirectURI string) url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {"opaque-state"},
		"resource":              {"https://curriculum.example/mcp"},
		"scope":                 {"mcp"},
		"code_challenge":        {strings.Repeat("a", 43)},
		"code_challenge_method": {"S256"},
	}
}
