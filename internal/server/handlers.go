package server

import (
	"encoding/json"
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
)

// handleHome renders the landing page.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":     "Home",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
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
