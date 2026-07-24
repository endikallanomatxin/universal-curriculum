package server

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type Server struct {
	Config      Config
	Database    *sql.DB
	Templates   *template.Template
	ObjectStore services.ObjectStore
	Handler     http.Handler
}

func Setup() (*Server, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	database, err := db.Open(config.PostgresConnString())
	if err != nil {
		return nil, err
	}
	templates, err := services.LoadTemplates()
	if err != nil {
		_ = database.Close()
		return nil, err
	}

	server := &Server{
		Config:      config,
		Database:    database,
		Templates:   templates,
		ObjectStore: services.NewLocalObjectStore(config.UploadsFolder),
	}
	if err := services.EnsureBootstrapAdmin(
		database,
		config.BootstrapAdminFullName,
		config.BootstrapAdminAlias,
		config.BootstrapAdminEmail,
		config.BootstrapAdminPassword,
	); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ensure bootstrap administrator: %w", err)
	}
	server.Handler = server.routes()
	return server, nil
}

func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("GET /learn", server.learn)
	mux.Handle("POST /learn/paths", requireUser(http.HandlerFunc(server.createLearningPath)))
	mux.Handle("POST /learn/paths/{id}", requireUser(http.HandlerFunc(server.updateLearningPath)))
	mux.Handle("POST /learn/paths/{id}/delete", requireUser(http.HandlerFunc(server.deleteLearningPath)))
	mux.Handle("POST /learn/units/{id}/completion", requireUser(http.HandlerFunc(server.setUnitCompletion)))
	mux.HandleFunc("GET /auth/login", server.login)
	mux.HandleFunc("POST /auth/login", server.login)
	mux.HandleFunc("POST /auth/logout", server.logout)
	mux.Handle("GET /account", requireUser(http.HandlerFunc(server.account)))
	mux.Handle("GET /admin/curriculum", server.requireAdmin(http.HandlerFunc(server.adminCurriculum)))
	mux.Handle("POST /admin/curriculum/proposals", server.requireAdmin(http.HandlerFunc(server.createCurriculumProposal)))
	mux.Handle("POST /admin/curriculum/proposals/{id}", server.requireAdmin(http.HandlerFunc(server.updateCurriculumProposal)))
	mux.Handle("POST /admin/curriculum/proposals/{id}/delete", server.requireAdmin(http.HandlerFunc(server.deleteCurriculumProposal)))
	mux.Handle("POST /admin/curriculum/proposals/{id}/publish", server.requireAdmin(http.HandlerFunc(server.publishCurriculumProposal)))
	mux.Handle("POST /admin/curriculum/proposals/{id}/changes/{changeID}/delete", server.requireAdmin(http.HandlerFunc(server.deleteCurriculumProposalChange)))
	mux.Handle("POST /admin/curriculum/units", server.requireAdmin(http.HandlerFunc(server.createCurriculumUnit)))
	mux.Handle("POST /admin/curriculum/units/{id}", server.requireAdmin(http.HandlerFunc(server.updateCurriculumUnit)))
	mux.Handle("POST /admin/curriculum/units/{id}/content", server.requireAdmin(http.HandlerFunc(server.updateCurriculumUnitContent)))
	mux.Handle("POST /admin/curriculum/units/{id}/delete", server.requireAdmin(http.HandlerFunc(server.deleteCurriculumUnit)))
	mux.Handle("POST /admin/curriculum/dependencies", server.requireAdmin(http.HandlerFunc(server.createUnitDependency)))
	mux.Handle("POST /admin/curriculum/dependencies/delete", server.requireAdmin(http.HandlerFunc(server.deleteUnitDependency)))
	mux.Handle("POST /admin/curriculum/proposals/{id}/revert", server.requireAdmin(http.HandlerFunc(server.revertCurriculumProposal)))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	return server.maintainSession(mux)
}

func (server *Server) health(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), time.Second)
	defer cancel()
	if err := server.Database.PingContext(ctx); err != nil {
		http.Error(writer, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

type userPageData struct {
	User           *models.User
	CSRFToken      string
	CurrentSection string
	Home           bool
}

func (server *Server) index(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "index.html", "home", true)
}

func (server *Server) account(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "account.html", "account", false)
}

func (server *Server) renderUserPage(writer http.ResponseWriter, request *http.Request, name, currentSection string, home bool) {
	data, err := server.loadUserPageData(request, currentSection, home)
	if err != nil {
		http.Error(writer, "Load user", http.StatusInternalServerError)
		return
	}
	server.render(writer, name, data)
}

func (server *Server) loadUserPageData(request *http.Request, currentSection string, home bool) (userPageData, error) {
	data := userPageData{CurrentSection: currentSection, Home: home}
	if userID, ok := services.SessionUserID(request); ok {
		user, err := db.GetUserByID(server.Database, userID)
		if err != nil {
			return data, err
		}
		data.User = user
		data.CSRFToken, _ = services.SessionCSRFToken(request)
	}
	return data, nil
}

func (server *Server) render(writer http.ResponseWriter, name string, data any) {
	server.renderStatus(writer, http.StatusOK, name, data)
}

func (server *Server) renderStatus(writer http.ResponseWriter, status int, name string, data any) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	if err := server.Templates.ExecuteTemplate(writer, name, data); err != nil {
		log.Printf("render template %s: %v", name, err)
	}
}

func (server *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.RunContext(ctx)
}

func (server *Server) RunContext(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:              server.Config.ServerAddress(),
		Handler:           server.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownContext)
	}()

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	if err := server.Database.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}
