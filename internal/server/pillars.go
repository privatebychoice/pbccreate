package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// --- management surface (top-level /pillars: define + coverage) ---

func (s *Server) handlePillarsList(w http.ResponseWriter, r *http.Request) {
	s.renderPillars(w, r, http.StatusOK, "")
}

func (s *Server) handlePillarCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	_, err := store.CreatePillar(r.Context(), s.db, channelID, r.PostFormValue("name"), r.PostFormValue("description"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/pillars", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidPillar):
		s.renderPillars(w, r, http.StatusBadRequest, "A channel and a name are required.")
	default:
		s.log.Error("create pillar", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handlePillarDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderPillarDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handlePillarUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdatePillar(r.Context(), s.db, id, r.PostFormValue("name"), r.PostFormValue("description"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/pillars/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidPillar):
		s.renderPillarDetail(w, r, id, http.StatusBadRequest, "A name is required.")
	case errors.Is(err, store.ErrPillarNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handlePillarDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeletePillar(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrPillarNotFound) {
		s.log.Error("delete pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/pillars", http.StatusSeeOther)
}

// --- assignment (on the content item) ---

// handleItemPillarAdd get-or-creates a pillar in the item's channel and assigns
// it to the item.
func (s *Server) handleItemPillarAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get content item for pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	pillar, err := store.GetOrCreatePillar(r.Context(), s.db, item.ChannelID, r.PostFormValue("name"))
	if errors.Is(err, store.ErrInvalidPillar) {
		s.redirectToItem(w, r, id) // name required (client-enforced)
		return
	}
	if err != nil {
		s.log.Error("get or create pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.AssignPillar(r.Context(), s.db, id, pillar.ID); err != nil {
		s.log.Error("assign pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleItemPillarRemove unassigns a pillar from the item (leaving it defined).
func (s *Server) handleItemPillarRemove(w http.ResponseWriter, r *http.Request) {
	id, pillarID, ok := s.requireContentItemAndSub(w, r, "pillarID")
	if !ok {
		return
	}
	if err := store.UnassignPillar(r.Context(), s.db, id, pillarID); err != nil {
		s.log.Error("unassign pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// --- render helpers ---

func (s *Server) renderPillars(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels for pillars", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	coverage, err := store.ListPillarCoverage(r.Context(), s.db)
	if err != nil {
		s.log.Error("list pillar coverage", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":       "Pillars",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"HasChannels": len(channels) > 0,
		"Coverage":    coverage,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "pillars.html.tmpl", data); err != nil {
		s.log.Error("render pillars", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderPillarDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	p, err := store.GetPillar(r.Context(), s.db, id)
	if errors.Is(err, store.ErrPillarNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get pillar", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Pillar: " + p.Name,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Pillar":    p,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "pillar_detail.html.tmpl", data); err != nil {
		s.log.Error("render pillar detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
