package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// shotView decorates a shot with its takes for the content detail page.
type shotView struct {
	store.Shot
	Takes []store.Take
}

// itemAndShot parses {id} (verifying the content item) and {shotID}, confirming
// the shot belongs to the item.
func (s *Server) itemAndShot(w http.ResponseWriter, r *http.Request) (itemID, shotID int64, ok bool) {
	itemID, ok = s.requireContentItem(w, r)
	if !ok {
		return 0, 0, false
	}
	shotID, err := strconv.ParseInt(r.PathValue("shotID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, 0, false
	}
	belongs, err := store.ShotExists(r.Context(), s.db, shotID, itemID)
	if err != nil {
		s.log.Error("check shot exists", "err", err, "id", itemID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return 0, 0, false
	}
	if !belongs {
		http.NotFound(w, r)
		return 0, 0, false
	}
	return itemID, shotID, true
}

// handleTakeAdd records a take against a shot.
func (s *Server) handleTakeAdd(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := s.itemAndShot(w, r)
	if !ok {
		return
	}
	rating, _ := strconv.Atoi(r.PostFormValue("rating"))
	mediaID, _ := strconv.ParseInt(r.PostFormValue("media_asset_id"), 10, 64)
	if err := store.AddTake(r.Context(), s.db, shotID, store.Take{
		MediaAssetID: mediaID,
		Label:        r.PostFormValue("label"),
		Rating:       rating,
		Circled:      r.PostFormValue("circled") != "",
		Notes:        r.PostFormValue("notes"),
	}); err != nil {
		s.log.Error("add take", "err", err, "shot", shotID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleTakeCircle toggles a take's circle marker.
func (s *Server) handleTakeCircle(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := s.itemAndShot(w, r)
	if !ok {
		return
	}
	takeID, err := strconv.ParseInt(r.PathValue("takeID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = store.ToggleTakeCircled(r.Context(), s.db, takeID, shotID)
	s.finishTake(w, r, id, err)
}

// handleTakeDelete removes a take.
func (s *Server) handleTakeDelete(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := s.itemAndShot(w, r)
	if !ok {
		return
	}
	takeID, err := strconv.ParseInt(r.PathValue("takeID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = store.DeleteTake(r.Context(), s.db, takeID, shotID)
	s.finishTake(w, r, id, err)
}

func (s *Server) finishTake(w http.ResponseWriter, r *http.Request, itemID int64, err error) {
	switch {
	case err == nil:
		s.redirectToItem(w, r, itemID)
	case errors.Is(err, store.ErrTakeNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("take op", "err", err, "id", itemID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
