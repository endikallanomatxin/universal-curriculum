package server

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/services"
)

const sessionCookieName = "session_token"

func (server *Server) maintainSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie(sessionCookieName)
		if errors.Is(err, http.ErrNoCookie) {
			next.ServeHTTP(writer, request)
			return
		}
		if err != nil || cookie.Value == "" {
			clearSessionCookie(writer, server.Config.IsProd())
			next.ServeHTTP(writer, request)
			return
		}

		use, err := db.UseSession(server.Database, cookie.Value)
		if errors.Is(err, db.ErrSessionNotFound) || errors.Is(err, db.ErrSessionExpired) {
			clearSessionCookie(writer, server.Config.IsProd())
			next.ServeHTTP(writer, request)
			return
		}
		if err != nil {
			log.Printf("maintain session: %v", err)
			http.Error(writer, "Unable to validate session", http.StatusInternalServerError)
			return
		}
		if use.RefreshCookie {
			token := cookie.Value
			if use.RotatedToken != "" {
				token = use.RotatedToken
				request.Header.Set("Cookie", replaceSessionCookie(request.Cookies(), token))
			}
			setSessionCookie(writer, token, server.Config.IsProd(), time.Now())
		}
		next.ServeHTTP(writer, services.WithSession(request, use.UserID, use.CSRFToken))
	})
}

func replaceSessionCookie(cookies []*http.Cookie, token string) string {
	values := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name == sessionCookieName {
			cookie.Value = token
		}
		values = append(values, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(values, "; ")
}

func requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := services.SessionUserID(request); !ok {
			http.Redirect(writer, request, "/auth/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
