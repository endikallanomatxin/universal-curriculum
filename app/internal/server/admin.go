package server

import (
	"log"
	"net/http"
	"net/url"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func (server *Server) requireAdmin(next http.Handler) http.Handler {
	return server.requireUserCapability(next, func(user *models.User) bool { return user.IsAdmin }, "administrator")
}

func (server *Server) requireContributor(next http.Handler) http.Handler {
	return server.requireUserCapability(next, func(user *models.User) bool { return user.CanContribute() }, "contributor")
}

func (server *Server) requireUserCapability(next http.Handler, allowed func(*models.User) bool, capability string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		userID, authenticated := services.SessionUserID(request)
		if !authenticated {
			http.Redirect(writer, request, "/auth/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		user, err := db.GetUserByID(server.Database, userID)
		if err != nil {
			log.Printf("authorize %s: %v", capability, err)
			http.Error(writer, "Unable to authorize account", http.StatusInternalServerError)
			return
		}
		if user == nil || !allowed(user) {
			http.Error(writer, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
