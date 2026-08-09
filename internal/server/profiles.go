package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// --- management surface (top-level /profiles: gear + location) ---

func (s *Server) handleProfilesList(w http.ResponseWriter, r *http.Request) {
	s.renderProfiles(w, r, http.StatusOK, "")
}

func (s *Server) handleProfileCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	_, err := store.CreateShootProfile(r.Context(), s.db, channelID,
		r.PostFormValue("kind"), r.PostFormValue("name"), r.PostFormValue("details"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/profiles", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidProfile):
		s.renderProfiles(w, r, http.StatusBadRequest, "A channel and a name are required.")
	default:
		s.log.Error("create shoot profile", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleProfileDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderProfileDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateShootProfile(r.Context(), s.db, id, r.PostFormValue("name"), r.PostFormValue("details"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/profiles/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidProfile):
		s.renderProfileDetail(w, r, id, http.StatusBadRequest, "A name is required.")
	case errors.Is(err, store.ErrProfileNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update shoot profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleProfileDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteShootProfile(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrProfileNotFound) {
		s.log.Error("delete shoot profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// --- assignment (on the content item) ---

// handleItemProfileAdd get-or-creates a profile of the posted kind in the item's
// channel and assigns it.
func (s *Server) handleItemProfileAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get content item for profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	profile, err := store.GetOrCreateShootProfile(r.Context(), s.db, item.ChannelID, r.PostFormValue("kind"), r.PostFormValue("name"))
	if errors.Is(err, store.ErrInvalidProfile) {
		s.redirectToItem(w, r, id) // name required (client-enforced)
		return
	}
	if err != nil {
		s.log.Error("get or create profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.AssignProfile(r.Context(), s.db, id, profile.ID); err != nil {
		s.log.Error("assign profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleItemProfileRemove unassigns a profile from the item (leaving it defined).
func (s *Server) handleItemProfileRemove(w http.ResponseWriter, r *http.Request) {
	id, profileID, ok := s.requireContentItemAndSub(w, r, "profileID")
	if !ok {
		return
	}
	if err := store.UnassignProfile(r.Context(), s.db, id, profileID); err != nil {
		s.log.Error("unassign profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// --- render helpers ---

func (s *Server) renderProfiles(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels for profiles", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	gear, err := store.ListShootProfiles(r.Context(), s.db, "gear")
	if err != nil {
		s.log.Error("list gear profiles", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	locations, err := store.ListShootProfiles(r.Context(), s.db, "location")
	if err != nil {
		s.log.Error("list location profiles", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":       "Profiles",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"HasChannels": len(channels) > 0,
		"Gear":        gear,
		"Locations":   locations,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "profiles.html.tmpl", data); err != nil {
		s.log.Error("render profiles", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderProfileDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	p, err := store.GetShootProfile(r.Context(), s.db, id)
	if errors.Is(err, store.ErrProfileNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get shoot profile", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Profile: " + p.Name,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Profile":   p,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "profile_detail.html.tmpl", data); err != nil {
		s.log.Error("render profile detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
