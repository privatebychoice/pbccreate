package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleMusicCueAdd records a cue on the item's cue sheet (SPEC §5.16).
func (s *Server) handleMusicCueAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	providerID, _ := strconv.ParseInt(r.PostFormValue("provider_id"), 10, 64)
	mediaID, _ := strconv.ParseInt(r.PostFormValue("media_asset_id"), 10, 64)
	_, err := store.AddMusicCue(r.Context(), s.db, store.MusicCue{
		ContentItemID: id,
		ProviderID:    providerID,
		MediaAssetID:  mediaID,
		Title:         r.PostFormValue("title"),
		Artist:        r.PostFormValue("artist"),
		InPoint:       r.PostFormValue("in_point"),
		OutPoint:      r.PostFormValue("out_point"),
		License:       r.PostFormValue("license"),
		Notes:         r.PostFormValue("notes"),
	})
	if err != nil && !errors.Is(err, store.ErrInvalidMusicCue) {
		s.log.Error("add music cue", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleMusicCueDelete removes a cue.
func (s *Server) handleMusicCueDelete(w http.ResponseWriter, r *http.Request) {
	id, cueID, ok := s.requireContentItemAndSub(w, r, "cueID")
	if !ok {
		return
	}
	err := store.DeleteMusicCue(r.Context(), s.db, cueID, id)
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrMusicCueNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("delete music cue", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleMusicCueToAttribution creates a music attribution (§5.11) from a cue,
// carrying its provider link and license so the credit reaches the description.
func (s *Server) handleMusicCueToAttribution(w http.ResponseWriter, r *http.Request) {
	id, cueID, ok := s.requireContentItemAndSub(w, r, "cueID")
	if !ok {
		return
	}
	cue, err := store.GetMusicCue(r.Context(), s.db, cueID, id)
	if errors.Is(err, store.ErrMusicCueNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get music cue for attribution", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if _, err := store.CreateAttribution(r.Context(), s.db, store.Attribution{
		ContentItemID:         id,
		Name:                  cue.Title,
		Kind:                  "music",
		Provider:              cue.Artist,
		ProviderID:            cue.ProviderID,
		License:               cue.License,
		MediaAssetID:          cue.MediaAssetID,
		IncludedInDescription: true,
	}); err != nil {
		s.log.Error("create attribution from cue", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}
