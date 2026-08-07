package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

const oauthScope = "mcp"

type OAuthClientMetadata struct {
	ClientID                          string   `json:"client_id"`
	ClientName                        string   `json:"client_name"`
	RedirectURIs                      []string `json:"redirect_uris"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	TokenEndpointAuthMethod           string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes                        []string `json:"grant_types,omitempty"`
	ResponseTypes                     []string `json:"response_types,omitempty"`
}

type OAuthClientMetadataResolver interface {
	Resolve(context.Context, string) (*OAuthClientMetadata, error)
}

type remoteOAuthClientMetadataResolver struct {
	client *http.Client
}

type oauthAuthorizationRequest struct {
	ClientID      string
	ClientName    string
	RedirectURI   string
	State         string
	Resource      string
	Scope         string
	CodeChallenge string
}

type oauthAuthorizationPageData struct {
	ClientName    string
	ClientID      string
	RedirectURI   string
	State         string
	Resource      string
	Scope         string
	CodeChallenge string
	CSRFToken     string
}

func (server *Server) oauthProtectedResourceMetadata(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writeOAuthJSON(writer, http.StatusOK, map[string]any{
		"resource":                 server.mcpResourceURL(),
		"authorization_servers":    []string{server.Config.AppBaseURL},
		"scopes_supported":         []string{oauthScope},
		"bearer_methods_supported": []string{"header"},
	})
}

func (server *Server) oauthAuthorizationServerMetadata(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writeOAuthJSON(writer, http.StatusOK, map[string]any{
		"issuer":                                         server.Config.AppBaseURL,
		"authorization_endpoint":                         server.Config.AppBaseURL + "/oauth/authorize",
		"token_endpoint":                                 server.Config.AppBaseURL + "/oauth/token",
		"revocation_endpoint":                            server.Config.AppBaseURL + "/oauth/revoke",
		"scopes_supported":                               []string{oauthScope},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"code_challenge_methods_supported":               []string{"S256"},
		"authorization_response_iss_parameter_supported": true,
		"client_id_metadata_document_supported":          true,
	})
}

func (server *Server) oauthAuthorize(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodPost {
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			http.Error(writer, "Invalid authorization request", http.StatusBadRequest)
			return
		}
	}
	userID, authenticated := services.SessionUserID(request)
	if !authenticated {
		if request.Method == http.MethodGet {
			http.Redirect(writer, request, "/auth/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
		} else {
			http.Error(writer, "Authentication required", http.StatusUnauthorized)
		}
		return
	}
	if request.Method == http.MethodPost && !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	input, err := server.validateOAuthAuthorizationRequest(request)
	if err != nil {
		http.Error(writer, "Invalid authorization request", http.StatusBadRequest)
		return
	}

	if request.Method == http.MethodGet {
		csrf, _ := services.SessionCSRFToken(request)
		server.render(writer, "oauth-authorize.html", oauthAuthorizationPageData{
			ClientName: input.ClientName, ClientID: input.ClientID,
			RedirectURI: input.RedirectURI, State: input.State, Resource: input.Resource,
			Scope: input.Scope, CodeChallenge: input.CodeChallenge, CSRFToken: csrf,
		})
		return
	}
	if request.FormValue("decision") != "allow" {
		server.redirectOAuthAuthorization(writer, request, input, "", "access_denied")
		return
	}
	code, err := db.CreateOAuthAuthorizationCode(server.Database, models.OAuthAuthorizationGrant{
		UserID: userID, ClientID: input.ClientID, RedirectURI: input.RedirectURI,
		Resource: input.Resource, Scope: input.Scope, CodeChallenge: input.CodeChallenge,
	})
	if err != nil {
		log.Printf("create OAuth authorization code: %v", err)
		http.Error(writer, "Unable to authorize client", http.StatusInternalServerError)
		return
	}
	server.redirectOAuthAuthorization(writer, request, input, code, "")
}

func (server *Server) validateOAuthAuthorizationRequest(request *http.Request) (*oauthAuthorizationRequest, error) {
	values := request.URL.Query()
	if request.Method == http.MethodPost {
		values = request.PostForm
	}
	input := &oauthAuthorizationRequest{
		ClientID: values.Get("client_id"), RedirectURI: values.Get("redirect_uri"),
		State: values.Get("state"), Resource: values.Get("resource"), Scope: values.Get("scope"),
		CodeChallenge: values.Get("code_challenge"),
	}
	if values.Get("response_type") != "code" || values.Get("code_challenge_method") != "S256" ||
		!validPKCEChallenge(input.CodeChallenge) || input.Resource != server.mcpResourceURL() || input.Scope != oauthScope {
		return nil, errors.New("unsupported OAuth parameters")
	}
	resolver := server.OAuthClients
	if resolver == nil {
		resolver = newRemoteOAuthClientMetadataResolver()
	}
	metadata, err := resolver.Resolve(request.Context(), input.ClientID)
	if err != nil {
		return nil, err
	}
	if metadata == nil || metadata.ClientID != input.ClientID || strings.TrimSpace(metadata.ClientName) == "" ||
		!slices.Contains(metadata.RedirectURIs, input.RedirectURI) {
		return nil, errors.New("client metadata does not match request")
	}
	if !supportsPublicClientAuthentication(metadata) {
		return nil, errors.New("only public OAuth clients are supported")
	}
	if len(metadata.GrantTypes) > 0 && !slices.Contains(metadata.GrantTypes, "authorization_code") {
		return nil, errors.New("client does not support authorization code grants")
	}
	if len(metadata.ResponseTypes) > 0 && !slices.Contains(metadata.ResponseTypes, "code") {
		return nil, errors.New("client does not support code responses")
	}
	if !validOAuthRedirectURI(input.RedirectURI) {
		return nil, errors.New("invalid redirect URI")
	}
	input.ClientName = strings.TrimSpace(metadata.ClientName)
	return input, nil
}

func supportsPublicClientAuthentication(metadata *OAuthClientMetadata) bool {
	if len(metadata.TokenEndpointAuthMethodsSupported) > 0 {
		return slices.Contains(metadata.TokenEndpointAuthMethodsSupported, "none")
	}
	return metadata.TokenEndpointAuthMethod == "none"
}

func (server *Server) redirectOAuthAuthorization(
	writer http.ResponseWriter, request *http.Request, input *oauthAuthorizationRequest, code, oauthError string,
) {
	redirect, _ := url.Parse(input.RedirectURI)
	query := redirect.Query()
	if oauthError != "" {
		query.Set("error", oauthError)
	} else {
		query.Set("code", code)
		query.Set("iss", server.Config.AppBaseURL)
	}
	if input.State != "" {
		query.Set("state", input.State)
	}
	redirect.RawQuery = query.Encode()
	http.Redirect(writer, request, redirect.String(), http.StatusFound)
}

func (server *Server) oauthToken(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		writeOAuthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	clientID := request.FormValue("client_id")
	resource := request.FormValue("resource")
	if !validCIMDClientID(clientID) || resource != server.mcpResourceURL() {
		writeOAuthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	var pair *models.OAuthTokenPair
	var err error
	switch request.FormValue("grant_type") {
	case "authorization_code":
		pair, err = services.ExchangeOAuthAuthorizationCode(
			server.Database, request.FormValue("code"), clientID,
			request.FormValue("redirect_uri"), resource, request.FormValue("code_verifier"),
		)
	case "refresh_token":
		pair, err = services.RefreshOAuthAccessToken(
			server.Database, request.FormValue("refresh_token"), clientID, resource,
		)
	default:
		writeOAuthError(writer, "unsupported_grant_type", http.StatusBadRequest)
		return
	}
	if errors.Is(err, services.ErrInvalidOAuthGrant) {
		writeOAuthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("exchange OAuth token: %v", err)
		writeOAuthError(writer, "server_error", http.StatusInternalServerError)
		return
	}
	writeOAuthJSON(writer, http.StatusOK, map[string]any{
		"access_token": pair.AccessToken, "token_type": "Bearer",
		"expires_in": pair.ExpiresIn, "refresh_token": pair.RefreshToken, "scope": pair.Scope,
	})
}

func (server *Server) oauthRevoke(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		writeOAuthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	clientID := request.FormValue("client_id")
	if !validCIMDClientID(clientID) {
		writeOAuthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	if err := db.RevokeOAuthToken(server.Database, request.FormValue("token"), clientID); err != nil {
		log.Printf("revoke OAuth token: %v", err)
		writeOAuthError(writer, "server_error", http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (server *Server) mcpResourceURL() string {
	return server.Config.AppBaseURL + "/mcp"
}

func writeOAuthError(writer http.ResponseWriter, code string, status int) {
	writeOAuthJSON(writer, status, map[string]string{"error": code})
}

func writeOAuthJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func newRemoteOAuthClientMetadataResolver() OAuthClientMetadataResolver {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, address := range addresses {
				if unsafeOAuthMetadataIP(address.IP) {
					continue
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
			}
			return nil, errors.New("client metadata host has no public address")
		},
	}
	client := &http.Client{Transport: transport, Timeout: 7 * time.Second}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !safeOAuthMetadataURL(request.URL) {
			return errors.New("unsafe client metadata redirect")
		}
		return nil
	}
	return &remoteOAuthClientMetadataResolver{client: client}
}

func (resolver *remoteOAuthClientMetadataResolver) Resolve(
	ctx context.Context, clientID string,
) (*OAuthClientMetadata, error) {
	if !validCIMDClientID(clientID) {
		return nil, errors.New("invalid client metadata URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := resolver.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch OAuth client metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch OAuth client metadata: status %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, errors.New("OAuth client metadata is not application/json")
	}
	var metadata OAuthClientMetadata
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode OAuth client metadata: %w", err)
	}
	if metadata.ClientID != clientID || strings.TrimSpace(metadata.ClientName) == "" || len(metadata.RedirectURIs) == 0 {
		return nil, errors.New("incomplete OAuth client metadata")
	}
	return &metadata, nil
}

func validCIMDClientID(clientID string) bool {
	parsed, err := url.Parse(clientID)
	return err == nil && safeOAuthMetadataURL(parsed) && parsed.User == nil && parsed.Fragment == ""
}

func validOAuthRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func validPKCEChallenge(value string) bool {
	if len(value) != 43 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func safeOAuthMetadataURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.Hostname() == "" || value.User != nil {
		return false
	}
	if ip := net.ParseIP(value.Hostname()); ip != nil {
		return !unsafeOAuthMetadataIP(ip)
	}
	return !strings.EqualFold(value.Hostname(), "localhost")
}

func unsafeOAuthMetadataIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
