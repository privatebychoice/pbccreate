package server

import (
	"errors"
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handlePublicationCreate records a per-platform publication for a content item
// (SPEC §5.12).
func (s *Server) handlePublicationCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	_, err := store.CreatePublication(r.Context(), s.db, publicationFromForm(r, 0, id))
	switch {
	case err == nil, errors.Is(err, store.ErrInvalidPublication):
		// Platform is required (client-enforced); on empty input just return.
		s.redirectToItem(w, r, id)
	default:
		s.log.Error("create publication", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handlePublicationUpdate edits an existing publication.
func (s *Server) handlePublicationUpdate(w http.ResponseWriter, r *http.Request) {
	id, pubID, ok := s.requireContentItemAndSub(w, r, "pubID")
	if !ok {
		return
	}
	err := store.UpdatePublication(r.Context(), s.db, publicationFromForm(r, pubID, id))
	switch {
	case err == nil, errors.Is(err, store.ErrInvalidPublication):
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrPublicationNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update publication", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handlePublicationDelete removes a publication.
func (s *Server) handlePublicationDelete(w http.ResponseWriter, r *http.Request) {
	id, pubID, ok := s.requireContentItemAndSub(w, r, "pubID")
	if !ok {
		return
	}
	if err := store.DeletePublication(r.Context(), s.db, pubID, id); err != nil && !errors.Is(err, store.ErrPublicationNotFound) {
		s.log.Error("delete publication", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// publicationFromForm builds a Publication from the request form. Field
// normalization (visibility, trimming) happens in the store layer.
func publicationFromForm(r *http.Request, id, contentItemID int64) store.Publication {
	return store.Publication{
		ID:             id,
		ContentItemID:  contentItemID,
		Platform:       r.PostFormValue("platform"),
		PublishedTitle: r.PostFormValue("published_title"),
		ExternalID:     r.PostFormValue("external_id"),
		URL:            r.PostFormValue("url"),
		OutputFile:     r.PostFormValue("output_file"),
		PostedOn:       r.PostFormValue("posted_on"),
		Visibility:     r.PostFormValue("visibility"),
		TagsSnapshot:   r.PostFormValue("tags_snapshot"),
		Notes:          r.PostFormValue("notes"),
	}
}
