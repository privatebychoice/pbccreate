package server

import (
	"errors"
	"net/http"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleChannelsList shows all channels plus the "new channel" form.
func (s *Server) handleChannelsList(w http.ResponseWriter, r *http.Request) {
	s.renderChannels(w, r, http.StatusOK, "")
}

// handleChannelsCreate handles the new-channel form submission (POST/redirect/GET
// on success; re-render with a 400 and message on validation failure).
func (s *Server) handleChannelsCreate(w http.ResponseWriter, r *http.Request) {
	_, err := store.CreateChannel(r.Context(), s.db, r.PostFormValue("name"), r.PostFormValue("kind"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/channels", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidChannel):
		s.renderChannels(w, r, http.StatusBadRequest, "Channel name is required.")
	default:
		s.log.Error("create channel", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderChannels lists channels and renders the page, optionally with an error
// banner, at the given status.
func (s *Server) renderChannels(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Channels",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Channels":  channels,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "channels.html.tmpl", data); err != nil {
		s.log.Error("render channels", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
