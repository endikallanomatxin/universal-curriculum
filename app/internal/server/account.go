package server

import (
	"errors"
	"log"
	"net/http"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type accountPageData struct {
	userPageData
	APITokens   []models.APIToken
	NewAPIToken string
	TokenError  string
}

func (server *Server) account(writer http.ResponseWriter, request *http.Request) {
	server.renderAccount(writer, request, http.StatusOK, "", "")
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
		server.renderAccount(writer, request, http.StatusBadRequest, "", err.Error())
		return
	}
	if err != nil {
		log.Printf("create API token: %v", err)
		http.Error(writer, "Unable to create API token", http.StatusInternalServerError)
		return
	}
	server.renderAccount(writer, request, http.StatusCreated, token.Token, "")
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
	server.renderStatus(writer, status, "account.html", accountPageData{
		userPageData: page,
		APITokens:    tokens, NewAPIToken: rawToken, TokenError: tokenError,
	})
}
