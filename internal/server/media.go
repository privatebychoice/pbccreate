package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"go.privatebychoice.com/pbccreate/internal/media"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleMediaAdd catalogues a media file by path, linking it to the content item
// (and optionally a shot). The path must be absolute and within a configured
// media root; on add we stat it to record size/mtime/presence.
func (s *Server) handleMediaAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}

	path := r.PostFormValue("path")
	if !media.WithinRoots(path, s.cfg.MediaRoots) {
		s.log.Warn("media add rejected: path outside roots", "id", id, "path", path)
		http.Error(w, "path must be an absolute path within a configured media root", http.StatusBadRequest)
		return
	}

	shotID, _ := strconv.ParseInt(r.PostFormValue("shot_id"), 10, 64)
	info := media.Stat(path)
	asset := store.MediaAsset{
		ContentItemID: id,
		ShotID:        shotID,
		Path:          path,
		Kind:          orAuto(r.PostFormValue("kind"), media.Kind(path)),
		Status:        r.PostFormValue("status"),
		Notes:         r.PostFormValue("notes"),
		Present:       info.Exists,
		SizeBytes:     info.Size,
		MTime:         info.ModTime,
	}
	if info.Exists {
		asset.LastSeenAt = info.ModTime
	}

	saved, err := store.AddMediaAsset(r.Context(), s.db, asset)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidMedia),
			errors.Is(err, store.ErrInvalidMediaKind),
			errors.Is(err, store.ErrInvalidMediaStatus):
			http.Error(w, "invalid media details", http.StatusBadRequest)
		default:
			s.log.Error("add media asset", "err", err, "id", id)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	// Best-effort probe on add: enrich with metadata when the file exists and
	// ffprobe is available. Failures never block the add.
	if saved.Present {
		s.probeMedia(r.Context(), id, saved)
	}
	s.redirectToItem(w, r, id)
}

// handleMediaStatus updates an asset's workflow status.
func (s *Server) handleMediaStatus(w http.ResponseWriter, r *http.Request) {
	id, mediaID, ok := s.requireContentItemAndSub(w, r, "mediaID")
	if !ok {
		return
	}
	err := store.UpdateMediaStatus(r.Context(), s.db, mediaID, id, r.PostFormValue("status"))
	s.finishMediaAction(w, r, id, err)
}

// handleMediaDelete removes an asset from the catalogue (leaving the file on disk).
func (s *Server) handleMediaDelete(w http.ResponseWriter, r *http.Request) {
	id, mediaID, ok := s.requireContentItemAndSub(w, r, "mediaID")
	if !ok {
		return
	}
	s.finishMediaAction(w, r, id, store.DeleteMediaAsset(r.Context(), s.db, mediaID, id))
}

// handleMediaVerify re-stats every asset for the item, flagging missing files.
func (s *Server) handleMediaVerify(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	assets, err := store.ListMediaAssets(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list media for verify", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	missing := 0
	for _, a := range assets {
		info := media.Stat(a.Path)
		if !info.Exists {
			missing++
		}
		if err := store.SetMediaPresence(r.Context(), s.db, a.ID, id, info.Exists, info.Size, info.ModTime); err != nil {
			s.log.Error("set media presence", "err", err, "id", id, "media", a.ID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	s.log.Info("media verified", "id", id, "checked", len(assets), "missing", missing)
	s.redirectToItem(w, r, id)
}

// handleMediaProbe runs ffprobe on every present asset for the item and stores
// the metadata. No-op (with a log) when ffprobe is unavailable.
func (s *Server) handleMediaProbe(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	if !media.ProbeAvailable(s.cfg.FFprobe) {
		s.log.Warn("media probe requested but ffprobe unavailable", "id", id)
		s.redirectToItem(w, r, id)
		return
	}
	assets, err := store.ListMediaAssets(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list media for probe", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	probed := 0
	for _, a := range assets {
		if !a.Present {
			continue
		}
		s.probeMedia(r.Context(), id, a)
		probed++
	}
	s.log.Info("media probed", "id", id, "probed", probed)
	s.redirectToItem(w, r, id)
}

// probeMedia runs ffprobe on one asset and stores the result. It is best-effort:
// a missing binary or probe error is logged (or ignored) but not fatal.
func (s *Server) probeMedia(ctx context.Context, contentItemID int64, a store.MediaAsset) {
	pctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	meta, err := media.Probe(pctx, s.cfg.FFprobe, a.Path)
	if err != nil {
		if !errors.Is(err, media.ErrProbeUnavailable) {
			s.log.Warn("ffprobe failed", "err", err, "path", a.Path)
		}
		return
	}
	if err := store.UpdateMediaMetadata(ctx, s.db, a.ID, contentItemID, store.MediaMetadata{
		DurationSeconds: meta.DurationSeconds,
		Width:           meta.Width,
		Height:          meta.Height,
		Codec:           meta.Codec,
		FPS:             meta.FPS,
		Container:       meta.Container,
	}); err != nil {
		s.log.Error("update media metadata", "err", err, "media", a.ID)
	}
}

// finishMediaAction maps a media store error to a response.
func (s *Server) finishMediaAction(w http.ResponseWriter, r *http.Request, id int64, err error) {
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrMediaNotFound):
		http.NotFound(w, r)
	case errors.Is(err, store.ErrInvalidMediaStatus):
		http.Error(w, "invalid status", http.StatusBadRequest)
	default:
		s.log.Error("media action", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// orAuto returns v if non-empty, otherwise the auto-detected fallback.
func orAuto(v, auto string) string {
	if v == "" {
		return auto
	}
	return v
}
