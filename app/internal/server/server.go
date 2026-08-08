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
	"sync"
	"syscall"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/mcpadapter"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type Server struct {
	Config       Config
	Database     *sql.DB
	Templates    *template.Template
	ObjectStore  services.ObjectStore
	EmailSender  services.EmailSender
	Handler      http.Handler
	OAuthClients OAuthClientMetadataResolver

	accountAPITokenFlashMu sync.Mutex
	accountAPITokenFlashes map[int64]accountAPITokenFlash
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
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", server.oauthProtectedResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", server.oauthProtectedResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", server.oauthAuthorizationServerMetadata)
	mux.HandleFunc("GET /oauth/authorize", server.oauthAuthorize)
	mux.HandleFunc("POST /oauth/authorize", server.oauthAuthorize)
	mux.HandleFunc("POST /oauth/token", server.oauthToken)
	mux.HandleFunc("POST /oauth/revoke", server.oauthRevoke)
	mcpHandler := mcpadapter.NewHandler(server.Database, server.Config.AppBaseURL)
	for _, method := range apiRequestMethods {
		mux.Handle(method+" /mcp", mcpHandler)
	}
	registerAPIRoute(mux, "/api", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiInfo)})
	registerAPIRoute(mux, "/api/{$}", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiInfo)})
	registerAPIRoute(mux, "/api/openapi.yaml", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiOpenAPI)})
	registerAPIRoute(mux, "/api/curriculum", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiGetCurriculum)})
	registerAPIRoute(mux, "/api/units", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiListUnits)})
	registerAPIRoute(mux, "/api/units/{unitId}", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiGetUnit)})
	registerAPIRoute(mux, "/api/curriculum/proposals", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiListAcceptedProposals)})
	registerAPIRoute(mux, "/api/curriculum/proposals/{proposalId}", map[string]http.Handler{http.MethodGet: http.HandlerFunc(server.apiGetAcceptedProposal)})
	registerAPIRoute(mux, "/api/learning-paths", map[string]http.Handler{
		http.MethodGet:  server.requireAPIToken(http.HandlerFunc(server.apiListLearningPaths)),
		http.MethodPost: server.requireAPIToken(http.HandlerFunc(server.apiCreateLearningPath)),
	})
	registerAPIRoute(mux, "/api/learning-paths/{pathId}", map[string]http.Handler{
		http.MethodGet:    server.requireAPIToken(http.HandlerFunc(server.apiGetLearningPath)),
		http.MethodPut:    server.requireAPIToken(http.HandlerFunc(server.apiUpdateLearningPath)),
		http.MethodDelete: server.requireAPIToken(http.HandlerFunc(server.apiDeleteLearningPath)),
	})
	registerAPIRoute(mux, "/api/recommendations", map[string]http.Handler{http.MethodGet: server.requireAPIToken(http.HandlerFunc(server.apiListRecommendations))})
	registerAPIRoute(mux, "/api/progress", map[string]http.Handler{http.MethodGet: server.requireAPIToken(http.HandlerFunc(server.apiGetProgress))})
	registerAPIRoute(mux, "/api/progress/{unitId}", map[string]http.Handler{http.MethodPut: server.requireAPIToken(http.HandlerFunc(server.apiSetProgress))})
	registerAPIRoute(mux, "/api/proposals", map[string]http.Handler{
		http.MethodGet:  server.requireAPIAdmin(http.HandlerFunc(server.apiListProposals)),
		http.MethodPost: server.requireAPIAdmin(http.HandlerFunc(server.apiCreateProposal)),
	})
	registerAPIRoute(mux, "/api/proposals/{proposalId}", map[string]http.Handler{
		http.MethodGet:    server.requireAPIAdmin(http.HandlerFunc(server.apiGetProposal)),
		http.MethodPut:    server.requireAPIAdmin(http.HandlerFunc(server.apiUpdateProposal)),
		http.MethodDelete: server.requireAPIAdmin(http.HandlerFunc(server.apiDeleteProposal)),
	})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/units", map[string]http.Handler{http.MethodPost: server.requireAPIAdmin(http.HandlerFunc(server.apiCreateProposalUnit))})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/units/{unitId}", map[string]http.Handler{
		http.MethodPut:    server.requireAPIAdmin(http.HandlerFunc(server.apiUpdateProposalUnit)),
		http.MethodDelete: server.requireAPIAdmin(http.HandlerFunc(server.apiDeleteProposalUnit)),
	})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/dependencies", map[string]http.Handler{http.MethodPost: server.requireAPIAdmin(http.HandlerFunc(server.apiAddProposalDependency))})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/dependencies/{unitId}/{prerequisiteId}", map[string]http.Handler{http.MethodDelete: server.requireAPIAdmin(http.HandlerFunc(server.apiRemoveProposalDependency))})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/recognitions", map[string]http.Handler{http.MethodPost: server.requireAPIAdmin(http.HandlerFunc(server.apiAddProposalRecognition))})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/changes/{changeId}", map[string]http.Handler{http.MethodDelete: server.requireAPIAdmin(http.HandlerFunc(server.apiDeleteProposalChange))})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/rebase", map[string]http.Handler{
		http.MethodGet:  server.requireAPIAdmin(http.HandlerFunc(server.apiGetProposalRebase)),
		http.MethodPost: server.requireAPIAdmin(http.HandlerFunc(server.apiResolveProposalRebase)),
	})
	registerAPIRoute(mux, "/api/proposals/{proposalId}/publish", map[string]http.Handler{http.MethodPost: server.requireAPIAdmin(http.HandlerFunc(server.apiPublishProposal))})
	for _, method := range apiRequestMethods {
		if method == http.MethodHead {
			continue
		}
		mux.HandleFunc(method+" /api/{path...}", server.apiNotFound)
	}
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("GET /about", server.about)
	mux.HandleFunc("GET /about/case", server.aboutCase)
	mux.HandleFunc("GET /about/proposal", server.aboutProposal)
	mux.HandleFunc("GET /about/documentation", server.documentation)
	mux.HandleFunc("GET /about/documentation/{slug}", server.documentation)
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
	mux.Handle("POST /account/oauth-connections/{id}/revoke", requireUser(http.HandlerFunc(server.revokeOAuthConnection)))
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
