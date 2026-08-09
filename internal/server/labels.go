package server

import (
	"errors"
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleLabelAdd get-or-creates an internal label in the item's channel and
// assigns it to the item.
func (s *Server) handleLabelAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get content item for label", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	label, err := store.GetOrCreateLabel(r.Context(), s.db, item.ChannelID, r.PostFormValue("name"), r.PostFormValue("color"))
	if errors.Is(err, store.ErrInvalidLabel) {
		s.redirectToItem(w, r, id) // name required (client-enforced)
		return
	}
	if err != nil {
		s.log.Error("get or create label", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.AssignLabel(r.Context(), s.db, id, label.ID); err != nil {
		s.log.Error("assign label", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleLabelRemove unassigns a label from the item (leaving it in the library).
func (s *Server) handleLabelRemove(w http.ResponseWriter, r *http.Request) {
	id, labelID, ok := s.requireContentItemAndSub(w, r, "labelID")
	if !ok {
		return
	}
	if err := store.UnassignLabel(r.Context(), s.db, id, labelID); err != nil {
		s.log.Error("unassign label", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}
