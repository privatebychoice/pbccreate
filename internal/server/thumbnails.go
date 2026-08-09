package server

import (
	"errors"
	"image/png"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
	"go.privatebychoice.com/pbccreate/internal/thumbnail"
)

// maxCanvasJSON caps the accepted canvas payload size (defensive; local tool).
const maxCanvasJSON = 256 * 1024

// handleThumbnailCreate creates a thumbnail (with a starter canvas) and opens its
// editor.
func (s *Server) handleThumbnailCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	canvasJSON, err := thumbnail.DefaultCanvas().JSON()
	if err != nil {
		s.log.Error("default canvas", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	th, err := store.CreateThumbnail(r.Context(), s.db, id, r.PostFormValue("name"), canvasJSON)
	switch {
	case err == nil:
		http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10)+"/thumbnails/"+strconv.FormatInt(th.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidThumbnail):
		s.redirectToItem(w, r, id) // name is required client-side
	default:
		s.log.Error("create thumbnail", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleThumbnailEdit renders the thumbnail editor.
func (s *Server) handleThumbnailEdit(w http.ResponseWriter, r *http.Request) {
	id, thumbID, ok := s.requireContentItemAndSub(w, r, "thumbID")
	if !ok {
		return
	}
	th, err := store.GetThumbnail(r.Context(), s.db, thumbID, id)
	if errors.Is(err, store.ErrThumbnailNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get thumbnail", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":         "Thumbnail: " + th.Name,
		"Build":         buildinfo.Build,
		"CSRFToken":     csrfToken(r),
		"ContentItemID": id,
		"ThumbID":       thumbID,
		"Name":          th.Name,
		"CanvasJSON":    th.CanvasJSON,
	}
	if err := s.tmpl.render(w, http.StatusOK, "thumbnail_edit.html.tmpl", data); err != nil {
		s.log.Error("render thumbnail edit", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleThumbnailSave stores the canvas produced by the browser editor. The
// payload is parsed into our model (dropping unknown fields) and re-encoded, so
// only known, valid data is persisted.
func (s *Server) handleThumbnailSave(w http.ResponseWriter, r *http.Request) {
	id, thumbID, ok := s.requireContentItemAndSub(w, r, "thumbID")
	if !ok {
		return
	}

	raw := r.PostFormValue("canvas_json")
	if len(raw) > maxCanvasJSON {
		http.Error(w, "canvas too large", http.StatusRequestEntityTooLarge)
		return
	}
	canvas, err := thumbnail.Parse(raw)
	if err != nil {
		http.Error(w, "invalid canvas", http.StatusBadRequest)
		return
	}
	canvasJSON, err := canvas.JSON()
	if err != nil {
		s.log.Error("encode canvas", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.UpdateThumbnailCanvas(r.Context(), s.db, thumbID, id, canvasJSON); err != nil {
		if errors.Is(err, store.ErrThumbnailNotFound) {
			http.NotFound(w, r)
			return
		}
		s.log.Error("update thumbnail", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10)+"/thumbnails/"+strconv.FormatInt(thumbID, 10), http.StatusSeeOther)
}

// handleThumbnailDelete removes a thumbnail.
func (s *Server) handleThumbnailDelete(w http.ResponseWriter, r *http.Request) {
	id, thumbID, ok := s.requireContentItemAndSub(w, r, "thumbID")
	if !ok {
		return
	}
	err := store.DeleteThumbnail(r.Context(), s.db, thumbID, id)
	if errors.Is(err, store.ErrThumbnailNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("delete thumbnail", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleThumbnailRender renders the thumbnail canvas to a PNG (the deterministic
// export authority; SPEC §5.5, §6).
func (s *Server) handleThumbnailRender(w http.ResponseWriter, r *http.Request) {
	id, thumbID, ok := s.requireContentItemAndSub(w, r, "thumbID")
	if !ok {
		return
	}
	th, err := store.GetThumbnail(r.Context(), s.db, thumbID, id)
	if errors.Is(err, store.ErrThumbnailNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get thumbnail for render", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	canvas, err := thumbnail.Parse(th.CanvasJSON)
	if err != nil {
		s.log.Error("parse canvas for render", "err", err, "thumb", thumbID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	img, err := thumbnail.Render(canvas)
	if err != nil {
		s.log.Error("render thumbnail", "err", err, "thumb", thumbID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	if err := png.Encode(w, img); err != nil {
		s.log.Error("encode png", "err", err, "thumb", thumbID)
	}
}
