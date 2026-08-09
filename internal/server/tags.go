package server

import (
	"errors"
	"net/http"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleTagAdd get-or-creates a keyword in the item's channel library and assigns
// it to the item.
func (s *Server) handleTagAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get content item for tag", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	tag, err := store.GetOrCreateTag(r.Context(), s.db, item.ChannelID, r.PostFormValue("name"))
	if errors.Is(err, store.ErrInvalidTag) {
		s.redirectToItem(w, r, id) // name required (client-enforced)
		return
	}
	if err != nil {
		s.log.Error("get or create tag", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.AssignTag(r.Context(), s.db, id, tag.ID); err != nil {
		s.log.Error("assign tag", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleTagRemove unassigns a tag from the item (leaving it in the library).
func (s *Server) handleTagRemove(w http.ResponseWriter, r *http.Request) {
	id, tagID, ok := s.requireContentItemAndSub(w, r, "tagID")
	if !ok {
		return
	}
	if err := store.UnassignTag(r.Context(), s.db, id, tagID); err != nil {
		s.log.Error("unassign tag", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// tagNamesCSV joins tag names as a comma-separated list (for copy + length hint).
func tagNamesCSV(tags []store.Tag) string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
