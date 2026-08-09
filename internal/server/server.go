// Package server hosts pbccreate's local web UI on loopback: routing, template
// rendering, security middleware, and lifecycle (see docs/SPEC.md §2, §6, §9).
package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.privatebychoice.com/pbccreate/internal/config"
	"go.privatebychoice.com/pbccreate/internal/web"
)

// Server wires configuration, storage, templates, and routes into an
// http.Handler.
type Server struct {
	cfg     *config.Config
	db      *sql.DB
	log     *slog.Logger
	tmpl    *templates
	handler http.Handler
}

// New constructs a Server, parsing the embedded templates and building the
// middleware-wrapped route handler.
func New(cfg *config.Config, db *sql.DB, log *slog.Logger) (*Server, error) {
	tmpl, err := parseTemplates(web.TemplatesFS())
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	s := &Server{cfg: cfg, db: db, log: log, tmpl: tmpl}
	s.handler = s.routes()
	return s, nil
}

// Handler exposes the fully wrapped handler (used by tests).
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(web.StaticFS())))
	mux.HandleFunc("GET /version", s.handleVersion)
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /channels", s.handleChannelsList)
	mux.HandleFunc("POST /channels", s.handleChannelsCreate)
	mux.HandleFunc("GET /content", s.handleContentBoard)
	mux.HandleFunc("POST /content", s.handleContentCreate)
	mux.HandleFunc("GET /content/{id}", s.handleContentDetail)
	mux.HandleFunc("POST /content/{id}/status", s.handleContentStatus)
	mux.HandleFunc("POST /content/{id}/script", s.handleContentScriptSave)
	mux.HandleFunc("POST /content/{id}/outline", s.handleOutlineAdd)
	mux.HandleFunc("POST /content/{id}/outline/{segID}/delete", s.handleOutlineDelete)
	mux.HandleFunc("POST /content/{id}/outline/{segID}/move", s.handleOutlineMove)
	mux.HandleFunc("POST /content/{id}/shots", s.handleShotAdd)
	mux.HandleFunc("POST /content/{id}/shots/{shotID}/status", s.handleShotStatus)
	mux.HandleFunc("POST /content/{id}/shots/{shotID}/delete", s.handleShotDelete)
	mux.HandleFunc("POST /content/{id}/shots/{shotID}/move", s.handleShotMove)
	mux.HandleFunc("POST /content/{id}/description", s.handleDescriptionSave)
	mux.HandleFunc("POST /content/{id}/description/chapters", s.handleDescriptionChapters)
	mux.HandleFunc("POST /content/{id}/media", s.handleMediaAdd)
	mux.HandleFunc("POST /content/{id}/media/verify", s.handleMediaVerify)
	mux.HandleFunc("POST /content/{id}/media/scan", s.handleMediaScan)
	mux.HandleFunc("POST /content/{id}/media/probe", s.handleMediaProbe)
	mux.HandleFunc("POST /content/{id}/media/previews", s.handleMediaPreviews)
	mux.HandleFunc("GET /content/{id}/media/{mediaID}/preview", s.handleMediaPreview)
	mux.HandleFunc("POST /content/{id}/media/{mediaID}/status", s.handleMediaStatus)
	mux.HandleFunc("POST /content/{id}/media/{mediaID}/delete", s.handleMediaDelete)
	// Middleware order: security headers outermost, then CSRF/same-origin.
	return s.securityHeaders(s.csrf(mux))
}

// Run starts the loopback HTTP server and blocks until ctx is cancelled, then
// shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	httpSrv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.Addr, err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()
	s.log.Info("web UI listening", "addr", s.cfg.Addr, "url", "http://"+s.cfg.Addr)

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.log.Info("shutting down web UI")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
