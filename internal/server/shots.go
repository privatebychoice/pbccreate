package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleShotAdd appends a shot to a content item's shot list.
func (s *Server) handleShotAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	shot := store.Shot{
		Description: r.PostFormValue("description"),
		Scene:       r.PostFormValue("scene"),
		Framing:     r.PostFormValue("framing"),
		Camera:      r.PostFormValue("camera"),
		Status:      r.PostFormValue("status"),
		Notes:       r.PostFormValue("notes"),
	}
	_, err := store.AddShot(r.Context(), s.db, id, shot)
	switch {
	case err == nil, errors.Is(err, store.ErrInvalidShot), errors.Is(err, store.ErrInvalidShotStatus):
		// Invalid input is normally blocked client-side; just return to the page.
		s.redirectToItem(w, r, id)
	default:
		s.log.Error("add shot", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleShotStatus updates a shot's production status.
func (s *Server) handleShotStatus(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := s.requireContentItemAndSub(w, r, "shotID")
	if !ok {
		return
	}
	err := store.UpdateShotStatus(r.Context(), s.db, shotID, id, r.PostFormValue("status"))
	s.finishShotAction(w, r, id, err)
}

// handleShotDelete removes a shot.
func (s *Server) handleShotDelete(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := s.requireContentItemAndSub(w, r, "shotID")
	if !ok {
		return
	}
	s.finishShotAction(w, r, id, store.DeleteShot(r.Context(), s.db, shotID, id))
}

// handleShotMove reorders a shot up or down.
func (s *Server) handleShotMove(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := s.requireContentItemAndSub(w, r, "shotID")
	if !ok {
		return
	}
	s.finishShotAction(w, r, id, store.MoveShot(r.Context(), s.db, shotID, id, r.PostFormValue("dir")))
}

// finishShotAction maps a shot store error to a response, redirecting to the
// item on success.
func (s *Server) finishShotAction(w http.ResponseWriter, r *http.Request, id int64, err error) {
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrShotNotFound):
		http.NotFound(w, r)
	case errors.Is(err, store.ErrInvalidShotStatus), errors.Is(err, store.ErrInvalidMove):
		http.Error(w, "invalid request", http.StatusBadRequest)
	default:
		s.log.Error("shot action", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// requireContentItemAndSub parses the {id} path value (verifying the item
// exists) and a sub-resource id path value (e.g. "shotID").
func (s *Server) requireContentItemAndSub(w http.ResponseWriter, r *http.Request, name string) (id, subID int64, ok bool) {
	id, ok = s.requireContentItem(w, r)
	if !ok {
		return 0, 0, false
	}
	subID, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, 0, false
	}
	return id, subID, true
}

func (s *Server) redirectToItem(w http.ResponseWriter, r *http.Request, id int64) {
	http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
