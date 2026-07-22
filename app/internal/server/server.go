package server

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"universal-curriculum/internal/db"
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
	server.Handler = server.routes()
	return server, nil
}

func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /", server.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	return mux
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

func (server *Server) index(writer http.ResponseWriter, _ *http.Request) {
	if err := server.Templates.ExecuteTemplate(writer, "index.html", nil); err != nil {
		http.Error(writer, "render page", http.StatusInternalServerError)
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
