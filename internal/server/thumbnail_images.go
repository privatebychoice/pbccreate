package server

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// Decoders registered for image.Decode at upload time. PNG is registered by
	// image/png (imported elsewhere in this package); add JPEG, GIF, and WebP.
	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"

	"go.privatebychoice.com/pbccreate/internal/store"
	"go.privatebychoice.com/pbccreate/internal/thumbnail"
)

const (
	maxThumbUpload  = 15 << 20   // 15 MiB per uploaded image
	maxThumbPixels  = 50_000_000 // reject decompression-bomb dimensions
	maxThumbDim     = 1920       // downscale so neither side exceeds this
	maxLayerDisplay = 800        // clamp an appended image layer's on-canvas size
)

// handleThumbnailImageUpload accepts an image upload, validates and normalizes it
// (decode-verify, dimension cap, downscale, re-encode to PNG which also strips
// metadata), stores it, and appends an image layer to the thumbnail's canvas.
func (s *Server) handleThumbnailImageUpload(w http.ResponseWriter, r *http.Request) {
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
		s.log.Error("get thumbnail for upload", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "no image file", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size > maxThumbUpload {
		http.Error(w, "image too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Dimension cap before a full decode (guards against decompression bombs).
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		http.Error(w, "unsupported or invalid image", http.StatusBadRequest)
		return
	}
	if cfg.Width*cfg.Height > maxThumbPixels {
		http.Error(w, "image dimensions too large", http.StatusBadRequest)
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		s.log.Error("seek upload", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	src, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "invalid image", http.StatusBadRequest)
		return
	}
	src = thumbnail.Fit(src, maxThumbDim)
	b := src.Bounds()
	fw, fh := b.Dx(), b.Dy()

	ti, err := store.CreateThumbnailImage(r.Context(), s.db, id, cleanFilename(header.Filename), fw, fh)
	if err != nil {
		s.log.Error("create thumbnail image", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.writeThumbImage(ti.ID, src); err != nil {
		s.log.Error("write thumbnail image", "err", err, "img", ti.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Append (or, for a background, prepend) an image layer to the canvas.
	canvas, err := thumbnail.Parse(th.CanvasJSON)
	if err != nil {
		s.log.Error("parse canvas for upload", "err", err, "thumb", thumbID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	layer := thumbnail.Layer{Type: "image", ImageID: ti.ID, W: fw, H: fh}
	if r.PostFormValue("as_background") != "" {
		layer.X, layer.Y, layer.W, layer.H = 0, 0, thumbnail.CanvasW, thumbnail.CanvasH
		canvas.Layers = append([]thumbnail.Layer{layer}, canvas.Layers...)
	} else {
		layer.W, layer.H = clampDisplay(fw, fh, maxLayerDisplay)
		canvas.Layers = append(canvas.Layers, layer)
	}
	canvasJSON, err := canvas.JSON()
	if err != nil {
		s.log.Error("encode canvas for upload", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.UpdateThumbnailCanvas(r.Context(), s.db, thumbID, id, canvasJSON); err != nil {
		s.log.Error("update canvas after upload", "err", err, "thumb", thumbID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10)+"/thumbnails/"+strconv.FormatInt(thumbID, 10), http.StatusSeeOther)
}

// handleThumbnailImageServe streams an uploaded thumbnail image.
func (s *Server) handleThumbnailImageServe(w http.ResponseWriter, r *http.Request) {
	id, imgID, ok := s.requireContentItemAndSub(w, r, "imgID")
	if !ok {
		return
	}
	ti, err := store.GetThumbnailImage(r.Context(), s.db, imgID, id)
	if errors.Is(err, store.ErrThumbnailImageNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get thumbnail image", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	path := s.thumbImagePath(ti.ID)
	if !withinDir(path, s.thumbImageDir()) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
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
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, filepath.Base(path), fi.ModTime(), f)
}

func (s *Server) writeThumbImage(id int64, img image.Image) error {
	if err := os.MkdirAll(s.thumbImageDir(), 0o700); err != nil {
		return fmt.Errorf("create thumb image dir: %w", err)
	}
	f, err := os.Create(s.thumbImagePath(id))
	if err != nil {
		return fmt.Errorf("create thumb image: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode thumb image: %w", err)
	}
	return nil
}

func (s *Server) thumbImageDir() string {
	return filepath.Join(s.cfg.DataDir, "thumbnails", "images")
}

func (s *Server) thumbImagePath(id int64) string {
	return filepath.Join(s.thumbImageDir(), fmt.Sprintf("img-%d.png", id))
}

// clampDisplay scales (w,h) down so neither exceeds maxSide, preserving aspect.
func clampDisplay(w, h, maxSide int) (int, int) {
	if w <= maxSide && h <= maxSide {
		return w, h
	}
	if w >= h {
		return maxSide, h * maxSide / w
	}
	return w * maxSide / h, maxSide
}

// cleanFilename reduces an upload filename to a safe, bounded display string.
func cleanFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "image"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}
