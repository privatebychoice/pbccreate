package server

import (
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleSearch runs a global search across scripts, ideas, notes, and metadata
// (SPEC §5.19) and renders grouped results.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	results, err := store.GlobalSearch(r.Context(), s.db, query)
	if err != nil {
		s.log.Error("global search", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Search",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Query":     query,
		"Results":   results,
		"Count":     len(results),
	}
	if err := s.tmpl.render(w, http.StatusOK, "search.html.tmpl", data); err != nil {
		s.log.Error("render search", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
