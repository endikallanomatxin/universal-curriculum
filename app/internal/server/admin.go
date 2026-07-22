package server

import (
	"log"
	"net/http"
	"net/url"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/services"
)

func (server *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		userID, authenticated := services.SessionUserID(request)
		if !authenticated {
			http.Redirect(writer, request, "/auth/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		user, err := db.GetUserByID(server.Database, userID)
		if err != nil {
			log.Printf("authorize administrator: %v", err)
			http.Error(writer, "Unable to authorize administrator", http.StatusInternalServerError)
			return
		}
		if user == nil || !user.IsAdmin {
			http.Error(writer, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
