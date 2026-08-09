package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

func (s *Server) handleFormatsList(w http.ResponseWriter, r *http.Request) {
	s.renderFormats(w, r, http.StatusOK, "")
}

func (s *Server) handleFormatCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	f, err := store.CreateFormat(r.Context(), s.db, store.Format{
		ChannelID:   channelID,
		Name:        r.PostFormValue("name"),
		Description: r.PostFormValue("description"),
		DefaultType: r.PostFormValue("default_type"),
		DefaultMode: r.PostFormValue("default_mode"),
	})
	switch {
	case err == nil:
		http.Redirect(w, r, "/formats/"+strconv.FormatInt(f.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidFormat):
		s.renderFormats(w, r, http.StatusBadRequest, "A channel and a name are required.")
	default:
		s.log.Error("create format", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleFormatDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderFormatDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleFormatUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateFormat(r.Context(), s.db, store.Format{
		ID:          id,
		Name:        r.PostFormValue("name"),
		Description: r.PostFormValue("description"),
		DefaultType: r.PostFormValue("default_type"),
		DefaultMode: r.PostFormValue("default_mode"),
	})
	switch {
	case err == nil:
		http.Redirect(w, r, "/formats/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidFormat):
		s.renderFormatDetail(w, r, id, http.StatusBadRequest, "A name is required.")
	case errors.Is(err, store.ErrFormatNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update format", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleFormatDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteFormat(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrFormatNotFound) {
		s.log.Error("delete format", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/formats", http.StatusSeeOther)
}

func (s *Server) handleFormatSegmentAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	target, _ := strconv.Atoi(r.PostFormValue("target_seconds"))
	err := store.AddFormatSegment(r.Context(), s.db, id, r.PostFormValue("title"), r.PostFormValue("notes"), target)
	if err != nil && !errors.Is(err, store.ErrInvalidFormatSegment) {
		s.log.Error("add format segment", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToFormat(w, r, id)
}

func (s *Server) handleFormatSegmentDelete(w http.ResponseWriter, r *http.Request) {
	id, segID, ok := formatAndSegment(w, r)
	if !ok {
		return
	}
	err := store.DeleteFormatSegment(r.Context(), s.db, segID, id)
	s.finishFormatSegment(w, r, id, err)
}

func (s *Server) handleFormatSegmentMove(w http.ResponseWriter, r *http.Request) {
	id, segID, ok := formatAndSegment(w, r)
	if !ok {
		return
	}
	err := store.MoveFormatSegment(r.Context(), s.db, segID, id, r.PostFormValue("dir"))
	if errors.Is(err, store.ErrInvalidMove) {
		http.Error(w, "invalid move", http.StatusBadRequest)
		return
	}
	s.finishFormatSegment(w, r, id, err)
}

func (s *Server) handleFormatShotAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.AddFormatShot(r.Context(), s.db, id, store.FormatShot{
		Description: r.PostFormValue("description"),
		Scene:       r.PostFormValue("scene"),
		Framing:     r.PostFormValue("framing"),
		Camera:      r.PostFormValue("camera"),
		Notes:       r.PostFormValue("notes"),
	})
	if err != nil && !errors.Is(err, store.ErrInvalidFormatShot) {
		s.log.Error("add format shot", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToFormat(w, r, id)
}

func (s *Server) handleFormatShotDelete(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := formatAndShot(w, r)
	if !ok {
		return
	}
	err := store.DeleteFormatShot(r.Context(), s.db, shotID, id)
	s.finishFormatShot(w, r, id, err)
}

func (s *Server) handleFormatShotMove(w http.ResponseWriter, r *http.Request) {
	id, shotID, ok := formatAndShot(w, r)
	if !ok {
		return
	}
	err := store.MoveFormatShot(r.Context(), s.db, shotID, id, r.PostFormValue("dir"))
	if errors.Is(err, store.ErrInvalidMove) {
		http.Error(w, "invalid move", http.StatusBadRequest)
		return
	}
	s.finishFormatShot(w, r, id, err)
}

func (s *Server) finishFormatShot(w http.ResponseWriter, r *http.Request, formatID int64, err error) {
	switch {
	case err == nil:
		s.redirectToFormat(w, r, formatID)
	case errors.Is(err, store.ErrFormatShotNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("format shot op", "err", err, "format", formatID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleFormatSeed creates a content item from the format and jumps to it.
func (s *Server) handleFormatSeed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	item, err := store.SeedContentItemFromFormat(r.Context(), s.db, id, r.PostFormValue("title"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/content/"+strconv.FormatInt(item.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrFormatNotFound):
		http.NotFound(w, r)
	case errors.Is(err, store.ErrInvalidContentItem):
		s.renderFormatDetail(w, r, id, http.StatusBadRequest, "A title is required to seed a content item.")
	default:
		s.log.Error("seed from format", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) finishFormatSegment(w http.ResponseWriter, r *http.Request, formatID int64, err error) {
	switch {
	case err == nil:
		s.redirectToFormat(w, r, formatID)
	case errors.Is(err, store.ErrFormatSegmentNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("format segment op", "err", err, "format", formatID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// --- render helpers ---

func (s *Server) renderFormats(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels for formats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	list, err := store.ListFormats(r.Context(), s.db)
	if err != nil {
		s.log.Error("list formats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":       "Formats",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"HasChannels": len(channels) > 0,
		"Formats":     list,
		"Types":       store.ContentTypes,
		"Modes":       store.CreatorModes,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "formats.html.tmpl", data); err != nil {
		s.log.Error("render formats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderFormatDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	f, err := store.GetFormat(r.Context(), s.db, id)
	if errors.Is(err, store.ErrFormatNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get format", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	segs, err := store.ListFormatSegments(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list format segments", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shots, err := store.ListFormatShots(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list format shots", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Format: " + f.Name,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Format":    f,
		"Segments":  segs,
		"Shots":     shots,
		"Types":     store.ContentTypes,
		"Modes":     store.CreatorModes,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "format_detail.html.tmpl", data); err != nil {
		s.log.Error("render format detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) redirectToFormat(w http.ResponseWriter, r *http.Request, id int64) {
	http.Redirect(w, r, "/formats/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func formatAndSegment(w http.ResponseWriter, r *http.Request) (formatID, segID int64, ok bool) {
	formatID, ok = pathInt64(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	segID, ok = pathInt64(w, r, "segID")
	if !ok {
		return 0, 0, false
	}
	return formatID, segID, true
}

func formatAndShot(w http.ResponseWriter, r *http.Request) (formatID, shotID int64, ok bool) {
	formatID, ok = pathInt64(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	shotID, ok = pathInt64(w, r, "shotID")
	if !ok {
		return 0, 0, false
	}
	return formatID, shotID, true
}
