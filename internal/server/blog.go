package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleRepurpose forks a source item into a derived blog (or jumps to the
// existing one) and redirects to the blog (SPEC §5.9).
func (s *Server) handleRepurpose(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get item for repurpose", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if item.Type == "blog" {
		// A blog is not itself repurposed to a blog; return to it.
		s.redirectToItem(w, r, id)
		return
	}
	// If a derived blog already exists, go there instead of making a second.
	if blogID, err := store.DerivedBlogID(r.Context(), s.db, id); err != nil {
		s.log.Error("derived blog lookup", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if blogID > 0 {
		s.redirectToItem(w, r, blogID)
		return
	}
	blogID, err := store.RepurposeToBlog(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("repurpose to blog", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, blogID)
}

// handleBlogReseed re-seeds a derived blog from its source, overwriting the
// blog's body, links, and tags (the form warns before this).
func (s *Server) handleBlogReseed(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	sourceID, err := store.DerivedSourceID(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("derived source lookup", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if sourceID == 0 {
		http.Error(w, "not a derived blog", http.StatusBadRequest)
		return
	}
	if err := store.SeedBlog(r.Context(), s.db, id, sourceID); err != nil {
		s.log.Error("reseed blog", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleBlogExport streams a blog item as a portable Markdown file.
func (s *Server) handleBlogExport(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if errors.Is(err, store.ErrContentItemNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get item for export", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if item.Type != "blog" {
		http.Error(w, "only blog items export to Markdown", http.StatusBadRequest)
		return
	}
	md, err := store.RenderBlogMarkdown(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("render blog markdown", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", blogSlug(item.Title)+".md"))
	_, _ = w.Write([]byte(md))
}

// blogSlug builds a filesystem-safe slug from a blog title for the .md filename.
func blogSlug(title string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	if slug := strings.Trim(b.String(), "-"); slug != "" {
		return slug
	}
	return "blog"
}
