package server

import (
	"errors"
	"image/png"
	"net/http"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
	"go.privatebychoice.com/pbccreate/internal/thumbnail"
)

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
	canvas, err := thumbnail.Parse(th.CanvasJSON)
	if err != nil {
		s.log.Error("parse canvas", "err", err, "thumb", thumbID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	title := firstTextLayer(canvas)

	data := map[string]any{
		"Title":         "Thumbnail: " + th.Name,
		"Build":         buildinfo.Build,
		"CSRFToken":     csrfToken(r),
		"ContentItemID": id,
		"ThumbID":       thumbID,
		"Name":          th.Name,
		"Background":    canvas.Background,
		"Text":          title.Text,
		"Color":         title.Color,
		"FontSize":      title.FontSize,
		"X":             title.X,
		"Y":             title.Y,
		"Bold":          title.Bold,
	}
	if err := s.tmpl.render(w, http.StatusOK, "thumbnail_edit.html.tmpl", data); err != nil {
		s.log.Error("render thumbnail edit", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleThumbnailSave writes the edited canvas (background + title layer).
func (s *Server) handleThumbnailSave(w http.ResponseWriter, r *http.Request) {
	id, thumbID, ok := s.requireContentItemAndSub(w, r, "thumbID")
	if !ok {
		return
	}

	canvas := thumbnail.Canvas{Background: orAuto(r.PostFormValue("background"), "#101418")}
	if text := r.PostFormValue("text"); strings.TrimSpace(text) != "" {
		size, _ := strconv.Atoi(r.PostFormValue("font_size"))
		x, _ := strconv.Atoi(r.PostFormValue("pos_x"))
		y, _ := strconv.Atoi(r.PostFormValue("pos_y"))
		canvas.Layers = []thumbnail.Layer{{
			Type:     "text",
			Text:     text,
			X:        x,
			Y:        y,
			FontSize: size,
			Color:    orAuto(r.PostFormValue("color"), "#ffffff"),
			Bold:     r.PostFormValue("bold") != "",
		}}
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

// firstTextLayer returns the first text layer, or a sensible default when the
// canvas has none (so the editor always shows title fields).
func firstTextLayer(c thumbnail.Canvas) thumbnail.Layer {
	for _, l := range c.Layers {
		if l.Type == "text" {
			return l
		}
	}
	return thumbnail.Layer{Type: "text", FontSize: 100, Color: "#ffffff", X: 80, Y: 280}
}
