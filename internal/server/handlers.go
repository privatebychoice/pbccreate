package server

import (
	"encoding/json"
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleHome renders the landing page.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	// Surface a setup nudge when the DaVinci project root has not been configured.
	_, source, err := store.ResolveProjectRoot(r.Context(), s.db, s.cfg.ProjectRoot)
	if err != nil {
		s.log.Error("resolve project root for home", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":            "Home",
		"Build":            buildinfo.Build,
		"CSRFToken":        csrfToken(r),
		"ProjectRootUnset": source == store.ProjectRootUnset,
	}
	if err := s.tmpl.render(w, http.StatusOK, "home.html.tmpl", data); err != nil {
		s.log.Error("render home", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleVersion reports the running version and build number (docs/SPEC.md §11).
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": buildinfo.Version,
		"build":   buildinfo.Build,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
