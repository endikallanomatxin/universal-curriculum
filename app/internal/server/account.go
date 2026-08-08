package server

import (
	"errors"
	"log"
	"net/http"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

const accountAPITokenFlashLifetime = 5 * time.Minute

type accountAPITokenFlash struct {
	NewToken string
	Error    string
	Expires  time.Time
}

type accountPageData struct {
	userPageData
	APITokens        []models.APIToken
	OAuthConnections []models.OAuthConnection
	NewAPIToken      string
	TokenError       string
}

func (server *Server) revokeOAuthConnection(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "Invalid form", http.StatusBadRequest)
		return
	}
	if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	connectionID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Connection not found", http.StatusNotFound)
		return
	}
	userID, _ := services.SessionUserID(request)
	deleted, err := db.DeleteOAuthConnection(server.Database, userID, connectionID)
	if err != nil {
		log.Printf("revoke OAuth connection: %v", err)
		http.Error(writer, "Unable to revoke connection", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(writer, "Connection not found", http.StatusNotFound)
		return
	}
	http.Redirect(writer, request, "/account", http.StatusSeeOther)
}

func (server *Server) account(writer http.ResponseWriter, request *http.Request) {
	userID, _ := services.SessionUserID(request)
	flash := server.takeAccountAPITokenFlash(userID)
	server.renderAccount(writer, request, http.StatusOK, flash.NewToken, flash.Error)
}

func (server *Server) createAPIToken(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "Invalid form", http.StatusBadRequest)
		return
	}
	if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	userID, _ := services.SessionUserID(request)
	token, err := services.CreateAPIToken(server.Database, userID, request.FormValue("name"))
	if errors.Is(err, services.ErrAPITokenNameRequired) || errors.Is(err, services.ErrAPITokenNameTooLong) {
		server.setAccountAPITokenFlash(userID, accountAPITokenFlash{Error: err.Error()})
		http.Redirect(writer, request, "/account", http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Printf("create API token: %v", err)
		http.Error(writer, "Unable to create API token", http.StatusInternalServerError)
		return
	}
	server.setAccountAPITokenFlash(userID, accountAPITokenFlash{NewToken: token.Token})
	http.Redirect(writer, request, "/account", http.StatusSeeOther)
}

func (server *Server) revokeAPIToken(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "Invalid form", http.StatusBadRequest)
		return
	}
	if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	tokenID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "API token not found", http.StatusNotFound)
		return
	}
	userID, _ := services.SessionUserID(request)
	if err := services.RevokeAPIToken(server.Database, userID, tokenID); errors.Is(err, services.ErrAPITokenNotFound) {
		http.Error(writer, "API token not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("revoke API token: %v", err)
		http.Error(writer, "Unable to revoke API token", http.StatusInternalServerError)
		return
	}
	http.Redirect(writer, request, "/account", http.StatusSeeOther)
}

func (server *Server) renderAccount(writer http.ResponseWriter, request *http.Request, status int, rawToken, tokenError string) {
	page, err := server.loadUserPageData(request, "account", false)
	if err != nil {
		log.Printf("load account user: %v", err)
		http.Error(writer, "Unable to load account", http.StatusInternalServerError)
		return
	}
	userID, _ := services.SessionUserID(request)
	tokens, err := db.ListAPITokens(server.Database, userID)
	if err != nil {
		log.Printf("list API tokens: %v", err)
		http.Error(writer, "Unable to load API tokens", http.StatusInternalServerError)
		return
	}
	connections, err := db.ListOAuthConnections(server.Database, userID)
	if err != nil {
		log.Printf("list OAuth connections: %v", err)
		http.Error(writer, "Unable to load connected apps", http.StatusInternalServerError)
		return
	}
	server.renderStatus(writer, status, "account.html", accountPageData{
		userPageData: page, APITokens: tokens, OAuthConnections: connections,
		NewAPIToken: rawToken, TokenError: tokenError,
	})
}

func (server *Server) setAccountAPITokenFlash(userID int64, flash accountAPITokenFlash) {
	now := time.Now()
	server.accountAPITokenFlashMu.Lock()
	defer server.accountAPITokenFlashMu.Unlock()
	if server.accountAPITokenFlashes == nil {
		server.accountAPITokenFlashes = make(map[int64]accountAPITokenFlash)
	}
	for existingUserID, existing := range server.accountAPITokenFlashes {
		if !existing.Expires.After(now) {
			delete(server.accountAPITokenFlashes, existingUserID)
		}
	}
	flash.Expires = now.Add(accountAPITokenFlashLifetime)
	server.accountAPITokenFlashes[userID] = flash
}

func (server *Server) takeAccountAPITokenFlash(userID int64) accountAPITokenFlash {
	server.accountAPITokenFlashMu.Lock()
	defer server.accountAPITokenFlashMu.Unlock()
	flash, ok := server.accountAPITokenFlashes[userID]
	delete(server.accountAPITokenFlashes, userID)
	if !ok || !flash.Expires.After(time.Now()) {
		return accountAPITokenFlash{}
	}
	return flash
}
