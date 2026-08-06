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
	EmailSender services.EmailSender
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
		EmailSender: services.NewResendEmailSender(config.ResendAPIKey, config.EmailFrom),
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
	mux.HandleFunc("GET /api", server.apiInfo)
	mux.HandleFunc("GET /api/{$}", server.apiInfo)
	mux.HandleFunc("POST /api", server.apiNotFound)
	mux.HandleFunc("PUT /api", server.apiNotFound)
	mux.HandleFunc("DELETE /api", server.apiNotFound)
	mux.HandleFunc("PATCH /api", server.apiNotFound)
	mux.HandleFunc("OPTIONS /api", server.apiNotFound)
	mux.HandleFunc("GET /api/openapi.yaml", server.apiOpenAPI)
	mux.HandleFunc("GET /api/curriculum", server.apiGetCurriculum)
	mux.HandleFunc("GET /api/units", server.apiListUnits)
	mux.HandleFunc("GET /api/units/{unitId}", server.apiGetUnit)
	mux.HandleFunc("GET /api/curriculum/proposals", server.apiListAcceptedProposals)
	mux.HandleFunc("GET /api/curriculum/proposals/{proposalId}", server.apiGetAcceptedProposal)
	mux.Handle("GET /api/learning-paths", server.requireAPIToken(http.HandlerFunc(server.apiListLearningPaths)))
	mux.Handle("POST /api/learning-paths", server.requireAPIToken(http.HandlerFunc(server.apiCreateLearningPath)))
	mux.Handle("GET /api/learning-paths/{pathId}", server.requireAPIToken(http.HandlerFunc(server.apiGetLearningPath)))
	mux.Handle("PUT /api/learning-paths/{pathId}", server.requireAPIToken(http.HandlerFunc(server.apiUpdateLearningPath)))
	mux.Handle("DELETE /api/learning-paths/{pathId}", server.requireAPIToken(http.HandlerFunc(server.apiDeleteLearningPath)))
	mux.Handle("GET /api/recommendations", server.requireAPIToken(http.HandlerFunc(server.apiListRecommendations)))
	mux.Handle("GET /api/progress", server.requireAPIToken(http.HandlerFunc(server.apiGetProgress)))
	mux.Handle("PUT /api/progress/{unitId}", server.requireAPIToken(http.HandlerFunc(server.apiSetProgress)))
	mux.Handle("GET /api/proposals", server.requireAPIAdmin(http.HandlerFunc(server.apiListProposals)))
	mux.Handle("POST /api/proposals", server.requireAPIAdmin(http.HandlerFunc(server.apiCreateProposal)))
	mux.Handle("GET /api/proposals/{proposalId}", server.requireAPIAdmin(http.HandlerFunc(server.apiGetProposal)))
	mux.Handle("PUT /api/proposals/{proposalId}", server.requireAPIAdmin(http.HandlerFunc(server.apiUpdateProposal)))
	mux.Handle("DELETE /api/proposals/{proposalId}", server.requireAPIAdmin(http.HandlerFunc(server.apiDeleteProposal)))
	mux.Handle("POST /api/proposals/{proposalId}/units", server.requireAPIAdmin(http.HandlerFunc(server.apiCreateProposalUnit)))
	mux.Handle("PUT /api/proposals/{proposalId}/units/{unitId}", server.requireAPIAdmin(http.HandlerFunc(server.apiUpdateProposalUnit)))
	mux.Handle("DELETE /api/proposals/{proposalId}/units/{unitId}", server.requireAPIAdmin(http.HandlerFunc(server.apiDeleteProposalUnit)))
	mux.Handle("POST /api/proposals/{proposalId}/dependencies", server.requireAPIAdmin(http.HandlerFunc(server.apiAddProposalDependency)))
	mux.Handle("DELETE /api/proposals/{proposalId}/dependencies", server.requireAPIAdmin(http.HandlerFunc(server.apiRemoveProposalDependency)))
	mux.Handle("POST /api/proposals/{proposalId}/recognitions", server.requireAPIAdmin(http.HandlerFunc(server.apiAddProposalRecognition)))
	mux.Handle("DELETE /api/proposals/{proposalId}/changes/{changeId}", server.requireAPIAdmin(http.HandlerFunc(server.apiDeleteProposalChange)))
	mux.Handle("GET /api/proposals/{proposalId}/rebase", server.requireAPIAdmin(http.HandlerFunc(server.apiGetProposalRebase)))
	mux.Handle("POST /api/proposals/{proposalId}/rebase", server.requireAPIAdmin(http.HandlerFunc(server.apiResolveProposalRebase)))
	mux.Handle("POST /api/proposals/{proposalId}/publish", server.requireAPIAdmin(http.HandlerFunc(server.apiPublishProposal)))
	mux.HandleFunc("GET /api/{path...}", server.apiNotFound)
	mux.HandleFunc("POST /api/{path...}", server.apiNotFound)
	mux.HandleFunc("PUT /api/{path...}", server.apiNotFound)
	mux.HandleFunc("DELETE /api/{path...}", server.apiNotFound)
	mux.HandleFunc("PATCH /api/{path...}", server.apiNotFound)
	mux.HandleFunc("OPTIONS /api/{path...}", server.apiNotFound)
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("GET /about", server.about)
	mux.HandleFunc("GET /about/case", server.aboutCase)
	mux.HandleFunc("GET /about/proposal", server.aboutProposal)
	mux.HandleFunc("GET /about/manifest", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/about/case", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("GET /license", server.license)
	mux.HandleFunc("GET /learn", server.learn)
	mux.Handle("POST /learn/paths", requireUser(http.HandlerFunc(server.createLearningPath)))
	mux.Handle("POST /learn/paths/{id}", requireUser(http.HandlerFunc(server.updateLearningPath)))
	mux.Handle("POST /learn/paths/{id}/delete", requireUser(http.HandlerFunc(server.deleteLearningPath)))
	mux.Handle("POST /learn/units/{id}/completion", requireUser(http.HandlerFunc(server.setUnitCompletion)))
	mux.HandleFunc("GET /auth/login", server.login)
	mux.HandleFunc("POST /auth/login", server.login)
	mux.HandleFunc("GET /auth/register", server.register)
	mux.HandleFunc("POST /auth/register", server.register)
	mux.HandleFunc("GET /auth/forgot-password", server.forgotPassword)
	mux.HandleFunc("POST /auth/forgot-password", server.forgotPassword)
	resetPasswordHandler := sensitiveAuthResponse(http.HandlerFunc(server.resetPassword))
	mux.Handle("GET /auth/reset-password", resetPasswordHandler)
	mux.Handle("POST /auth/reset-password", resetPasswordHandler)
	mux.HandleFunc("POST /auth/logout", server.logout)
	mux.Handle("GET /account", requireUser(http.HandlerFunc(server.account)))
	mux.Handle("POST /account/api-tokens", requireUser(sensitiveAuthResponse(http.HandlerFunc(server.createAPIToken))))
	mux.Handle("POST /account/api-tokens/{id}/revoke", requireUser(http.HandlerFunc(server.revokeAPIToken)))
	mux.Handle("GET /curriculum-modification", server.requireAdmin(http.HandlerFunc(server.curriculumModification)))
	mux.Handle("POST /curriculum-modification/proposals", server.requireAdmin(http.HandlerFunc(server.createCurriculumProposal)))
	mux.Handle("POST /curriculum-modification/proposals/{id}", server.requireAdmin(http.HandlerFunc(server.updateCurriculumProposal)))
	mux.Handle("POST /curriculum-modification/proposals/{id}/delete", server.requireAdmin(http.HandlerFunc(server.deleteCurriculumProposal)))
	mux.Handle("POST /curriculum-modification/proposals/{id}/publish", server.requireAdmin(http.HandlerFunc(server.publishCurriculumProposal)))
	mux.Handle("POST /curriculum-modification/proposals/{id}/rebase", server.requireAdmin(http.HandlerFunc(server.rebaseCurriculumProposal)))
	mux.Handle("POST /curriculum-modification/proposals/{id}/changes/{changeID}/delete", server.requireAdmin(http.HandlerFunc(server.deleteCurriculumProposalChange)))
	mux.Handle("POST /curriculum-modification/units", server.requireAdmin(http.HandlerFunc(server.createCurriculumUnit)))
	mux.Handle("POST /curriculum-modification/units/{id}", server.requireAdmin(http.HandlerFunc(server.updateCurriculumUnit)))
	mux.Handle("POST /curriculum-modification/units/{id}/content", server.requireAdmin(http.HandlerFunc(server.updateCurriculumUnitContent)))
	mux.Handle("POST /curriculum-modification/units/{id}/delete", server.requireAdmin(http.HandlerFunc(server.deleteCurriculumUnit)))
	mux.Handle("POST /curriculum-modification/dependencies", server.requireAdmin(http.HandlerFunc(server.createUnitDependency)))
	mux.Handle("POST /curriculum-modification/dependencies/delete", server.requireAdmin(http.HandlerFunc(server.deleteUnitDependency)))
	mux.Handle("POST /curriculum-modification/recognitions", server.requireAdmin(http.HandlerFunc(server.createCurriculumRecognition)))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	return configureClientIP(server, server.maintainSession(mux))
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
	User            *models.User
	CSRFToken       string
	CurrentSection  string
	Home            bool
	Recommendations []homeLearningPathRecommendation
}

func (server *Server) index(writer http.ResponseWriter, request *http.Request) {
	data, err := server.loadUserPageData(request, "home", true)
	if err != nil {
		log.Printf("load home user: %v", err)
		http.Error(writer, "Unable to load home", http.StatusInternalServerError)
		return
	}
	if data.User != nil {
		data.Recommendations, err = server.homeLearningRecommendations(data.User.ID)
		if err != nil {
			log.Printf("load home recommendations: %v", err)
			http.Error(writer, "Unable to load recommendations", http.StatusInternalServerError)
			return
		}
	}
	server.render(writer, "index.html", data)
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
