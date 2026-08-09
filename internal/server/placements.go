package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// placementView pairs a placement with its deliverable checklist for rendering.
type placementView struct {
	store.Placement
	Deliverables []store.Deliverable
}

// handlePlacementCreate attaches a campaign to the content item.
func (s *Server) handlePlacementCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	campaignID, _ := strconv.ParseInt(r.PostFormValue("campaign_id"), 10, 64)
	if campaignID <= 0 {
		http.Error(w, "a campaign is required", http.StatusBadRequest)
		return
	}
	_, err := store.CreatePlacement(r.Context(), s.db, campaignID, id, r.PostFormValue("deadline"))
	switch {
	case err == nil, errors.Is(err, store.ErrPlacementExists):
		s.redirectToItem(w, r, id)
	default:
		s.log.Error("create placement", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handlePlacementDelete detaches a campaign from the content item.
func (s *Server) handlePlacementDelete(w http.ResponseWriter, r *http.Request) {
	id, pid, ok := s.requireContentItemAndSub(w, r, "pid")
	if !ok {
		return
	}
	err := store.DeletePlacement(r.Context(), s.db, pid, id)
	if err != nil && !errors.Is(err, store.ErrPlacementNotFound) {
		s.log.Error("delete placement", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleDeliverableAdd appends a checklist item to a placement.
func (s *Server) handleDeliverableAdd(w http.ResponseWriter, r *http.Request) {
	id, p, ok := s.requirePlacement(w, r)
	if !ok {
		return
	}
	_, err := store.AddDeliverable(r.Context(), s.db, p.ID, r.PostFormValue("description"))
	if err != nil && !errors.Is(err, store.ErrInvalidDeliverable) {
		s.log.Error("add deliverable", "err", err, "placement", p.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleDeliverableToggle flips a deliverable's done state.
func (s *Server) handleDeliverableToggle(w http.ResponseWriter, r *http.Request) {
	id, p, ok := s.requirePlacement(w, r)
	if !ok {
		return
	}
	did, ok := pathInt64(w, r, "did")
	if !ok {
		return
	}
	s.finishDeliverable(w, r, id, store.ToggleDeliverable(r.Context(), s.db, did, p.ID))
}

// handleDeliverableDelete removes a deliverable.
func (s *Server) handleDeliverableDelete(w http.ResponseWriter, r *http.Request) {
	id, p, ok := s.requirePlacement(w, r)
	if !ok {
		return
	}
	did, ok := pathInt64(w, r, "did")
	if !ok {
		return
	}
	s.finishDeliverable(w, r, id, store.DeleteDeliverable(r.Context(), s.db, did, p.ID))
}

func (s *Server) finishDeliverable(w http.ResponseWriter, r *http.Request, id int64, err error) {
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrDeliverableNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("deliverable action", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// requirePlacement resolves the content item and a placement scoped to it.
func (s *Server) requirePlacement(w http.ResponseWriter, r *http.Request) (int64, store.Placement, bool) {
	id, pid, ok := s.requireContentItemAndSub(w, r, "pid")
	if !ok {
		return 0, store.Placement{}, false
	}
	p, err := store.GetPlacement(r.Context(), s.db, pid, id)
	if errors.Is(err, store.ErrPlacementNotFound) {
		http.NotFound(w, r)
		return 0, store.Placement{}, false
	}
	if err != nil {
		s.log.Error("get placement", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return 0, store.Placement{}, false
	}
	return id, p, true
}
