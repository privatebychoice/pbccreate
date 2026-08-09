package server

import (
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleRetrospectiveSave upserts the item's post-publish retrospective (§5.12).
func (s *Server) handleRetrospectiveSave(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	if _, err := store.SaveRetrospective(r.Context(), s.db, store.Retrospective{
		ContentItemID:    id,
		WhatWorked:       r.PostFormValue("what_worked"),
		ToImprove:        r.PostFormValue("to_improve"),
		PerformanceNotes: r.PostFormValue("performance_notes"),
		ReviewedOn:       r.PostFormValue("reviewed_on"),
	}); err != nil {
		s.log.Error("save retrospective", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}
