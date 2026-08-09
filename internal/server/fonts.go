package server

import (
	"net/http"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// The thumbnail canvas editor uses the same Go fonts as the server-side render
// authority (internal/thumbnail), so the on-screen preview matches the exported
// PNG. We serve the embedded font bytes rather than committing binary files.

func (s *Server) handleFontRegular(w http.ResponseWriter, r *http.Request) {
	serveFont(w, goregular.TTF)
}

func (s *Server) handleFontBold(w http.ResponseWriter, r *http.Request) {
	serveFont(w, gobold.TTF)
}

func serveFont(w http.ResponseWriter, ttf []byte) {
	w.Header().Set("Content-Type", "font/ttf")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(ttf)
}
