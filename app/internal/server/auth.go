package server

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

const invalidLoginMessage = "Incorrect email or password."

type authPageData struct {
	Error string
	Next  string
}

func (server *Server) login(writer http.ResponseWriter, request *http.Request) {
	if _, authenticated := services.SessionUserID(request); authenticated && request.Method == http.MethodGet {
		http.Redirect(writer, request, "/account", http.StatusSeeOther)
		return
	}

	switch request.Method {
	case http.MethodGet:
		server.render(writer, "login.html", authPageData{Next: safeRedirectPath(request.URL.Query().Get("next"), "/account")})
	case http.MethodPost:
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "login.html", authPageData{Error: invalidLoginMessage})
			return
		}
		email := models.NormalizeEmail(request.FormValue("email"))
		password := request.FormValue("password")
		next := safeRedirectPath(request.FormValue("next"), "/account")
		if email == "" || password == "" {
			server.renderStatus(writer, http.StatusUnauthorized, "login.html", authPageData{Error: invalidLoginMessage, Next: next})
			return
		}

		user, err := services.AuthenticateLocal(server.Database, email, password)
		if errors.Is(err, services.ErrInvalidCredentials) {
			server.renderStatus(writer, http.StatusUnauthorized, "login.html", authPageData{Error: invalidLoginMessage, Next: next})
			return
		}
		if err != nil {
			log.Printf("authenticate local user: %v", err)
			http.Error(writer, "Unable to log in", http.StatusInternalServerError)
			return
		}
		token, err := db.CreateSession(server.Database, user.ID)
		if err != nil {
			log.Printf("create session: %v", err)
			http.Error(writer, "Unable to log in", http.StatusInternalServerError)
			return
		}
		setSessionCookie(writer, token, server.Config.IsProd(), time.Now())
		http.Redirect(writer, request, next, http.StatusSeeOther)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (server *Server) logout(writer http.ResponseWriter, request *http.Request) {
	if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if cookie, err := request.Cookie(sessionCookieName); err == nil {
		if err := db.DeleteSession(server.Database, cookie.Value); err != nil {
			log.Printf("delete session: %v", err)
			http.Error(writer, "Unable to log out", http.StatusInternalServerError)
			return
		}
	}
	clearSessionCookie(writer, server.Config.IsProd())
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func safeRedirectPath(value, fallback string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fallback
	}
	return value
}

func setSessionCookie(writer http.ResponseWriter, token string, secure bool, now time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/",
		Expires: now.Add(db.SessionIdleTimeout), MaxAge: int(db.SessionIdleTimeout.Seconds()),
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(writer http.ResponseWriter, secure bool) {
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Path: "/", Expires: time.Unix(0, 0), MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}
