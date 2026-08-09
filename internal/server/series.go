package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

func (s *Server) handleSeriesList(w http.ResponseWriter, r *http.Request) {
	s.renderSeries(w, r, http.StatusOK, "")
}

func (s *Server) handleSeriesCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	series, err := store.CreateSeries(r.Context(), s.db, channelID, r.PostFormValue("name"), r.PostFormValue("description"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/series/"+strconv.FormatInt(series.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidSeries):
		s.renderSeries(w, r, http.StatusBadRequest, "A channel and a name are required.")
	default:
		s.log.Error("create series", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSeriesDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderSeriesDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleSeriesUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateSeries(r.Context(), s.db, id, r.PostFormValue("name"), r.PostFormValue("description"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/series/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidSeries):
		s.renderSeriesDetail(w, r, id, http.StatusBadRequest, "A name is required.")
	case errors.Is(err, store.ErrSeriesNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update series", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSeriesDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteSeries(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrSeriesNotFound) {
		s.log.Error("delete series", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/series", http.StatusSeeOther)
}

func (s *Server) handleEpisodeAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	contentItemID, _ := strconv.ParseInt(r.PostFormValue("content_item_id"), 10, 64)
	err := store.AddEpisode(r.Context(), s.db, id, contentItemID)
	switch {
	case err == nil,
		errors.Is(err, store.ErrInvalidEpisode),
		errors.Is(err, store.ErrEpisodeExists),
		errors.Is(err, store.ErrItemChannelDiffer):
		// Bad selections are normally prevented client-side; just return.
		s.redirectToSeries(w, r, id)
	case errors.Is(err, store.ErrSeriesNotFound), errors.Is(err, store.ErrContentItemNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("add episode", "err", err, "series", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleEpisodeArc(w http.ResponseWriter, r *http.Request) {
	id, epID, ok := seriesAndEpisode(w, r)
	if !ok {
		return
	}
	err := store.UpdateEpisodeArc(r.Context(), s.db, epID, id, r.PostFormValue("arc_notes"))
	s.finishEpisode(w, r, id, err)
}

func (s *Server) handleEpisodeRemove(w http.ResponseWriter, r *http.Request) {
	id, epID, ok := seriesAndEpisode(w, r)
	if !ok {
		return
	}
	err := store.RemoveEpisode(r.Context(), s.db, epID, id)
	s.finishEpisode(w, r, id, err)
}

func (s *Server) handleEpisodeMove(w http.ResponseWriter, r *http.Request) {
	id, epID, ok := seriesAndEpisode(w, r)
	if !ok {
		return
	}
	err := store.MoveEpisode(r.Context(), s.db, epID, id, r.PostFormValue("dir"))
	if errors.Is(err, store.ErrInvalidMove) {
		http.Error(w, "invalid move", http.StatusBadRequest)
		return
	}
	s.finishEpisode(w, r, id, err)
}

// finishEpisode maps the common episode-mutation outcomes to a response.
func (s *Server) finishEpisode(w http.ResponseWriter, r *http.Request, seriesID int64, err error) {
	switch {
	case err == nil:
		s.redirectToSeries(w, r, seriesID)
	case errors.Is(err, store.ErrEpisodeNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("episode op", "err", err, "series", seriesID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// --- render helpers ---

func (s *Server) renderSeries(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels for series", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	list, err := store.ListSeries(r.Context(), s.db)
	if err != nil {
		s.log.Error("list series", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":       "Series",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"HasChannels": len(channels) > 0,
		"Series":      list,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "series.html.tmpl", data); err != nil {
		s.log.Error("render series", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderSeriesDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	series, err := store.GetSeries(r.Context(), s.db, id)
	if errors.Is(err, store.ErrSeriesNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get series", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	episodes, err := store.ListEpisodes(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list episodes", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	candidates, err := s.episodeCandidates(r, series.ChannelID, episodes)
	if err != nil {
		s.log.Error("episode candidates", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	done := 0
	for _, e := range episodes {
		if e.Status == "published" {
			done++
		}
	}
	data := map[string]any{
		"Title":      "Series: " + series.Name,
		"Build":      buildinfo.Build,
		"CSRFToken":  csrfToken(r),
		"Series":     series,
		"Episodes":   episodes,
		"Candidates": candidates,
		"DoneCount":  done,
		"Error":      errMsg,
	}
	if err := s.tmpl.render(w, status, "series_detail.html.tmpl", data); err != nil {
		s.log.Error("render series detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// episodeCandidates returns the channel's content items not already in the series.
func (s *Server) episodeCandidates(r *http.Request, channelID int64, episodes []store.Episode) ([]store.ContentItem, error) {
	items, err := store.ListContentItems(r.Context(), s.db)
	if err != nil {
		return nil, err
	}
	inSeries := make(map[int64]bool, len(episodes))
	for _, e := range episodes {
		inSeries[e.ContentItemID] = true
	}
	var out []store.ContentItem
	for _, it := range items {
		if it.ChannelID == channelID && !inSeries[it.ID] {
			out = append(out, it)
		}
	}
	return out, nil
}

func (s *Server) redirectToSeries(w http.ResponseWriter, r *http.Request, id int64) {
	http.Redirect(w, r, "/series/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// seriesAndEpisode parses the {id} and {epID} path values.
func seriesAndEpisode(w http.ResponseWriter, r *http.Request) (seriesID, episodeID int64, ok bool) {
	seriesID, ok = pathInt64(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	episodeID, ok = pathInt64(w, r, "epID")
	if !ok {
		return 0, 0, false
	}
	return seriesID, episodeID, true
}
