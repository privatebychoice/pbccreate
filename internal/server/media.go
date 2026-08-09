package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// Best-effort enrichment on add when the file exists: probe metadata, then
	// generate a preview frame. Failures never block the add.
	if saved.Present {
		meta := s.probeMedia(r.Context(), id, saved)
		s.generatePreview(r.Context(), id, saved, meta.DurationSeconds)
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
	// Capture the preview path so we can clean it up after a successful delete.
	asset, _ := store.GetMediaAsset(r.Context(), s.db, mediaID, id)
	err := store.DeleteMediaAsset(r.Context(), s.db, mediaID, id)
	if err == nil && asset.PreviewPath != "" {
		_ = os.Remove(asset.PreviewPath)
	}
	s.finishMediaAction(w, r, id, err)
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

// handleMediaScan walks a directory (within a media root) and catalogues any
// recognized media files not already tracked for this item. Cataloguing only —
// metadata/previews are produced by the Probe / Generate previews actions.
func (s *Server) handleMediaScan(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	dir := strings.TrimSpace(r.PostFormValue("dir"))
	if !media.WithinRoots(dir, s.cfg.MediaRoots) {
		http.Error(w, "folder must be an absolute path within a configured media root", http.StatusBadRequest)
		return
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
		return
	}

	files, err := media.ScanDir(dir)
	if err != nil {
		s.log.Error("scan dir", "err", err, "dir", dir)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	existing, err := s.existingMediaPaths(r.Context(), id)
	if err != nil {
		s.log.Error("list media for scan", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	added := 0
	for _, f := range files {
		if existing[f.Path] {
			continue
		}
		if _, err := store.AddMediaAsset(r.Context(), s.db, store.MediaAsset{
			ContentItemID: id,
			Path:          f.Path,
			Kind:          f.Kind,
			Status:        "recorded",
			Present:       true,
			SizeBytes:     f.Size,
			MTime:         f.ModTime,
			LastSeenAt:    f.ModTime,
		}); err != nil {
			s.log.Warn("scan add failed", "err", err, "path", f.Path)
			continue
		}
		added++
	}
	skipped := len(files) - added
	s.log.Info("media scan complete", "id", id, "found", len(files), "added", added, "skipped", skipped)
	http.Redirect(w, r, fmt.Sprintf("/content/%d?added=%d&skipped=%d", id, added, skipped), http.StatusSeeOther)
}

// existingMediaPaths returns the set of paths already catalogued for an item.
func (s *Server) existingMediaPaths(ctx context.Context, contentItemID int64) (map[string]bool, error) {
	assets, err := store.ListMediaAssets(ctx, s.db, contentItemID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(assets))
	for _, a := range assets {
		set[a.Path] = true
	}
	return set, nil
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

// probeMedia runs ffprobe on one asset and stores the result, returning the
// metadata (zero on failure). Best-effort: a missing binary or probe error is
// logged (or ignored) but not fatal.
func (s *Server) probeMedia(ctx context.Context, contentItemID int64, a store.MediaAsset) media.Metadata {
	pctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	meta, err := media.Probe(pctx, s.cfg.FFprobe, a.Path)
	if err != nil {
		if !errors.Is(err, media.ErrProbeUnavailable) {
			s.log.Warn("ffprobe failed", "err", err, "path", a.Path)
		}
		return media.Metadata{}
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
	return meta
}

// handleMediaPreviews generates preview frames for every present video/image
// asset. No-op (with a log) when ffmpeg is unavailable.
func (s *Server) handleMediaPreviews(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	if !media.ThumbAvailable(s.cfg.FFmpeg) {
		s.log.Warn("preview generation requested but ffmpeg unavailable", "id", id)
		s.redirectToItem(w, r, id)
		return
	}
	assets, err := store.ListMediaAssets(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list media for previews", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	generated := 0
	for _, a := range assets {
		if a.Present && s.generatePreview(r.Context(), id, a, a.DurationSeconds) {
			generated++
		}
	}
	s.log.Info("media previews generated", "id", id, "generated", generated)
	s.redirectToItem(w, r, id)
}

// generatePreview creates and stores a preview frame for one asset. Returns true
// when a preview was written. Best-effort: unsupported kinds and a missing binary
// are silent; other errors are logged.
func (s *Server) generatePreview(ctx context.Context, contentItemID int64, a store.MediaAsset, durationSeconds int) bool {
	dest := s.previewPath(a.ID)
	seek := 0
	if durationSeconds > 2 {
		seek = 1
	}
	tctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := media.GenerateThumbnail(tctx, s.cfg.FFmpeg, a.Path, dest, a.Kind, seek); err != nil {
		if !errors.Is(err, media.ErrThumbUnavailable) && !errors.Is(err, media.ErrThumbUnsupportedKind) {
			s.log.Warn("thumbnail failed", "err", err, "path", a.Path)
		}
		return false
	}
	if err := store.SetMediaPreview(ctx, s.db, a.ID, contentItemID, dest); err != nil {
		s.log.Error("set media preview", "err", err, "media", a.ID)
		return false
	}
	return true
}

// handleMediaPreview serves an asset's cached preview image.
func (s *Server) handleMediaPreview(w http.ResponseWriter, r *http.Request) {
	id, mediaID, ok := s.requireContentItemAndSub(w, r, "mediaID")
	if !ok {
		return
	}
	asset, err := store.GetMediaAsset(r.Context(), s.db, mediaID, id)
	if errors.Is(err, store.ErrMediaNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get media asset for preview", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Only serve from within our previews dir (defense in depth: the path is
	// generated by us, not user input).
	if asset.PreviewPath == "" || !withinDir(asset.PreviewPath, s.previewDir()) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(asset.PreviewPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(asset.PreviewPath), fi.ModTime(), f)
}

func (s *Server) previewDir() string {
	return filepath.Join(s.cfg.DataDir, "previews")
}

func (s *Server) previewPath(assetID int64) string {
	return filepath.Join(s.previewDir(), fmt.Sprintf("media-%d.jpg", assetID))
}

// withinDir reports whether path resolves to a location inside dir.
func withinDir(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
