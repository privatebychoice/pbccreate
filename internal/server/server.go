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
	// Serve the Go fonts (used by the canvas editor to match the render output);
	// these patterns are more specific than "GET /static/" so they take priority.
	mux.HandleFunc("GET /static/fonts/go-regular.ttf", s.handleFontRegular)
	mux.HandleFunc("GET /static/fonts/go-bold.ttf", s.handleFontBold)
	mux.HandleFunc("GET /version", s.handleVersion)
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /channels", s.handleChannelsList)
	mux.HandleFunc("POST /channels", s.handleChannelsCreate)
	mux.HandleFunc("GET /sponsors", s.handleSponsorsList)
	mux.HandleFunc("POST /sponsors", s.handleSponsorCreate)
	mux.HandleFunc("GET /sponsors/{id}", s.handleSponsorDetail)
	mux.HandleFunc("POST /sponsors/{id}", s.handleSponsorUpdate)
	mux.HandleFunc("POST /sponsors/{id}/delete", s.handleSponsorDelete)
	mux.HandleFunc("POST /sponsors/{id}/campaigns", s.handleCampaignCreate)
	mux.HandleFunc("GET /sponsors/{id}/campaigns/{cid}", s.handleCampaignEdit)
	mux.HandleFunc("POST /sponsors/{id}/campaigns/{cid}", s.handleCampaignUpdate)
	mux.HandleFunc("POST /sponsors/{id}/campaigns/{cid}/delete", s.handleCampaignDelete)
	mux.HandleFunc("GET /providers", s.handleProvidersList)
	mux.HandleFunc("POST /providers", s.handleProviderCreate)
	mux.HandleFunc("GET /providers/{id}", s.handleProviderDetail)
	mux.HandleFunc("POST /providers/{id}", s.handleProviderUpdate)
	mux.HandleFunc("POST /providers/{id}/delete", s.handleProviderDelete)
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
	mux.HandleFunc("POST /content/{id}/description/sponsor", s.handleDescriptionSponsor)
	mux.HandleFunc("POST /content/{id}/description/hashtags", s.handleDescriptionHashtags)
	mux.HandleFunc("POST /content/{id}/description/credits", s.handleDescriptionCredits)
	mux.HandleFunc("POST /content/{id}/attributions", s.handleAttributionAdd)
	mux.HandleFunc("POST /content/{id}/attributions/{attrID}/toggle", s.handleAttributionToggle)
	mux.HandleFunc("POST /content/{id}/attributions/{attrID}/delete", s.handleAttributionDelete)
	mux.HandleFunc("POST /content/{id}/tags", s.handleTagAdd)
	mux.HandleFunc("POST /content/{id}/tags/{tagID}/remove", s.handleTagRemove)
	mux.HandleFunc("POST /content/{id}/labels", s.handleLabelAdd)
	mux.HandleFunc("POST /content/{id}/labels/{labelID}/remove", s.handleLabelRemove)
	mux.HandleFunc("POST /content/{id}/placements", s.handlePlacementCreate)
	mux.HandleFunc("POST /content/{id}/placements/{pid}/delete", s.handlePlacementDelete)
	mux.HandleFunc("POST /content/{id}/placements/{pid}/deliverables", s.handleDeliverableAdd)
	mux.HandleFunc("POST /content/{id}/placements/{pid}/deliverables/{did}/toggle", s.handleDeliverableToggle)
	mux.HandleFunc("POST /content/{id}/placements/{pid}/deliverables/{did}/delete", s.handleDeliverableDelete)
	mux.HandleFunc("POST /content/{id}/thumbnails", s.handleThumbnailCreate)
	mux.HandleFunc("GET /content/{id}/thumbnails/{thumbID}", s.handleThumbnailEdit)
	mux.HandleFunc("GET /content/{id}/thumbnails/{thumbID}/render.png", s.handleThumbnailRender)
	mux.HandleFunc("POST /content/{id}/thumbnails/{thumbID}", s.handleThumbnailSave)
	mux.HandleFunc("POST /content/{id}/thumbnails/{thumbID}/delete", s.handleThumbnailDelete)
	mux.HandleFunc("POST /content/{id}/thumbnails/{thumbID}/images", s.handleThumbnailImageUpload)
	mux.HandleFunc("GET /content/{id}/thumbnail-images/{imgID}", s.handleThumbnailImageServe)
	mux.HandleFunc("POST /content/{id}/media", s.handleMediaAdd)
	mux.HandleFunc("POST /content/{id}/media/verify", s.handleMediaVerify)
	mux.HandleFunc("POST /content/{id}/media/scan", s.handleMediaScan)
	mux.HandleFunc("POST /content/{id}/media/probe", s.handleMediaProbe)
	mux.HandleFunc("POST /content/{id}/media/previews", s.handleMediaPreviews)
	mux.HandleFunc("GET /content/{id}/media/{mediaID}/preview", s.handleMediaPreview)
	mux.HandleFunc("POST /content/{id}/media/{mediaID}/status", s.handleMediaStatus)
	mux.HandleFunc("POST /content/{id}/media/{mediaID}/delete", s.handleMediaDelete)
	// Middleware order: security headers outermost, then a request-body size
	// cap (before form parsing), then CSRF/same-origin.
	return s.securityHeaders(s.limitBody(s.csrf(mux)))
}

// maxRequestBody caps any request body before parsing (uploads are validated
// more tightly in their handler).
const maxRequestBody = 16 << 20

// limitBody bounds request bodies for methods that carry one, so form/multipart
// parsing (including in the CSRF middleware) can never read an unbounded body.
func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
			}
		}
		next.ServeHTTP(w, r)
	})
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
