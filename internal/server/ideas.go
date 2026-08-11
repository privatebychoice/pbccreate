package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleIdeasList shows the capture form and the scored idea backlog (§5.13).
func (s *Server) handleIdeasList(w http.ResponseWriter, r *http.Request) {
	s.renderIdeas(w, r, http.StatusOK, "")
}

func (s *Server) handleIdeaCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	idea := ideaFromForm(r, 0)
	idea.ChannelID = channelID
	_, err := store.CreateIdea(r.Context(), s.db, idea)
	switch {
	case err == nil:
		http.Redirect(w, r, "/ideas", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidIdea):
		s.renderIdeas(w, r, http.StatusBadRequest, "A channel and a title are required.")
	case errors.Is(err, store.ErrPillarNotFound):
		s.renderIdeas(w, r, http.StatusBadRequest, "That pillar is not in the selected channel.")
	default:
		s.log.Error("create idea", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleIdeaDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderIdeaDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleIdeaUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	idea := ideaFromForm(r, id)
	err := store.UpdateIdea(r.Context(), s.db, idea)
	switch {
	case err == nil:
		http.Redirect(w, r, "/ideas/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidIdea):
		s.renderIdeaDetail(w, r, id, http.StatusBadRequest, "A title is required.")
	case errors.Is(err, store.ErrPillarNotFound):
		s.renderIdeaDetail(w, r, id, http.StatusBadRequest, "That pillar is not in this idea's channel.")
	case errors.Is(err, store.ErrIdeaNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update idea", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleIdeaDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteIdea(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrIdeaNotFound) {
		s.log.Error("delete idea", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/ideas", http.StatusSeeOther)
}

// handleIdeaPromote turns an idea into a ContentItem (entering the pipeline at
// status "idea"), links it back to the idea, and jumps to the new item.
func (s *Server) handleIdeaPromote(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	idea, err := store.GetIdea(r.Context(), s.db, id)
	if errors.Is(err, store.ErrIdeaNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get idea for promote", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	item, err := store.CreateContentItem(r.Context(), s.db, idea.ChannelID,
		r.PostFormValue("type"), r.PostFormValue("mode"), idea.Title)
	if errors.Is(err, store.ErrInvalidContentItem) {
		s.renderIdeaDetail(w, r, id, http.StatusBadRequest, "Pick a valid content type to promote.")
		return
	}
	if err != nil {
		s.log.Error("promote idea: create content item", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.MarkIdeaPromoted(r.Context(), s.db, id, item.ID); err != nil {
		s.log.Error("mark idea promoted", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Carry the idea's pillar onto the new content item so the theme follows it.
	if idea.PillarID != 0 {
		if err := store.AssignPillar(r.Context(), s.db, item.ID, idea.PillarID); err != nil {
			s.log.Warn("promote idea: assign pillar", "err", err, "idea", id, "pillar", idea.PillarID)
		}
	}
	http.Redirect(w, r, "/content/"+strconv.FormatInt(item.ID, 10), http.StatusSeeOther)
}

// --- render helpers ---

func (s *Server) renderIdeas(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels for ideas", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ideas, err := store.ListIdeas(r.Context(), s.db)
	if err != nil {
		s.log.Error("list ideas", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":       "Ideas",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"HasChannels": len(channels) > 0,
		"Ideas":       ideas,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "ideas.html.tmpl", data); err != nil {
		s.log.Error("render ideas", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderIdeaDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	idea, err := store.GetIdea(r.Context(), s.db, id)
	if errors.Is(err, store.ErrIdeaNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get idea", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	pillars, err := store.ListPillarsForChannel(r.Context(), s.db, idea.ChannelID)
	if err != nil {
		s.log.Error("list pillars for idea", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Idea: " + idea.Title,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Idea":      idea,
		"Statuses":  store.IdeaStatuses,
		"Types":     store.ContentTypes,
		"Modes":     store.CreatorModes,
		"Pillars":   pillars,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "idea_detail.html.tmpl", data); err != nil {
		s.log.Error("render idea detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// ideaFromForm builds an Idea from the request form. Validation/clamping of the
// ICE factors and status happens in the store layer.
func ideaFromForm(r *http.Request, id int64) store.Idea {
	impact, _ := strconv.Atoi(r.PostFormValue("impact"))
	confidence, _ := strconv.Atoi(r.PostFormValue("confidence"))
	effort, _ := strconv.Atoi(r.PostFormValue("effort"))
	return store.Idea{
		ID:         id,
		Title:      r.PostFormValue("title"),
		Note:       r.PostFormValue("note"),
		Source:     r.PostFormValue("source"),
		Impact:     impact,
		Confidence: confidence,
		Effort:     effort,
		Status:     r.PostFormValue("status"),
		PillarID:   formID(r, "pillar_id"),
	}
}
