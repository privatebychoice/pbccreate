package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// statusColumn is one pipeline column on the board.
type statusColumn struct {
	Status string
	Items  []store.ContentItem
}

// handleContentBoard shows the create form and the pipeline board.
func (s *Server) handleContentBoard(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, http.StatusOK, "")
}

// handleContentCreate handles the new content-item form.
func (s *Server) handleContentCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	_, err := store.CreateContentItem(r.Context(), s.db, channelID,
		r.PostFormValue("type"), r.PostFormValue("mode"), r.PostFormValue("title"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/content", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidContentItem):
		s.renderContent(w, r, http.StatusBadRequest, "A channel, a title, and a valid type are required.")
	default:
		s.log.Error("create content item", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderContent loads channels and content items, groups items into pipeline
// columns, and renders the board.
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items, err := store.ListContentItems(r.Context(), s.db)
	if err != nil {
		s.log.Error("list content items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":       "Content",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"Types":       store.ContentTypes,
		"Modes":       store.CreatorModes,
		"Columns":     groupByStatus(items),
		"HasChannels": len(channels) > 0,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "content.html.tmpl", data); err != nil {
		s.log.Error("render content", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// groupByStatus buckets items into columns following the canonical status order.
// Items with an unrecognized status are dropped from the board (should not occur
// given the schema CHECK, but keeps the view robust).
func groupByStatus(items []store.ContentItem) []statusColumn {
	byStatus := make(map[string][]store.ContentItem, len(store.ContentStatuses))
	for _, it := range items {
		byStatus[it.Status] = append(byStatus[it.Status], it)
	}
	cols := make([]statusColumn, 0, len(store.ContentStatuses))
	for _, st := range store.ContentStatuses {
		cols = append(cols, statusColumn{Status: st, Items: byStatus[st]})
	}
	return cols
}
