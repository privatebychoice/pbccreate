package server

import (
	"errors"
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleTitleCandidateAdd records a candidate title for the item.
func (s *Server) handleTitleCandidateAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	err := store.AddTitleCandidate(r.Context(), s.db, id, r.PostFormValue("text"))
	if err != nil && !errors.Is(err, store.ErrInvalidTitleCandidate) {
		s.log.Error("add title candidate", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleTitleCandidateChoose marks a candidate as the chosen title.
func (s *Server) handleTitleCandidateChoose(w http.ResponseWriter, r *http.Request) {
	id, candID, ok := s.requireContentItemAndSub(w, r, "candID")
	if !ok {
		return
	}
	err := store.ChooseTitleCandidate(r.Context(), s.db, candID, id)
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrTitleCandidateNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("choose title candidate", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleTitleCandidateDelete removes a candidate.
func (s *Server) handleTitleCandidateDelete(w http.ResponseWriter, r *http.Request) {
	id, candID, ok := s.requireContentItemAndSub(w, r, "candID")
	if !ok {
		return
	}
	err := store.DeleteTitleCandidate(r.Context(), s.db, candID, id)
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrTitleCandidateNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("delete title candidate", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleSwipeAdd records a title-swipe pattern in the item's channel.
func (s *Server) handleSwipeAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get content item for swipe", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	err = store.AddSwipe(r.Context(), s.db, item.ChannelID, r.PostFormValue("pattern"), r.PostFormValue("note"))
	if err != nil && !errors.Is(err, store.ErrInvalidSwipe) {
		s.log.Error("add swipe", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleSwipeDelete removes a swipe entry from the item's channel.
func (s *Server) handleSwipeDelete(w http.ResponseWriter, r *http.Request) {
	id, swipeID, ok := s.requireContentItemAndSub(w, r, "swipeID")
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get content item for swipe delete", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	err = store.DeleteSwipe(r.Context(), s.db, swipeID, item.ChannelID)
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrSwipeNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("delete swipe", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
