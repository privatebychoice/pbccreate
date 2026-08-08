// Package web holds pbccreate's embedded front-end assets — HTML templates and
// static files (CSS/JS/fonts). Embedding them keeps the binary self-contained
// and every front-end resource self-hosted (see docs/SPEC.md §6).
package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates static
var files embed.FS

// TemplatesFS returns the embedded HTML templates, rooted at the templates dir.
func TemplatesFS() fs.FS { return sub("templates") }

// StaticFS returns the embedded static assets, rooted at the static dir.
func StaticFS() fs.FS { return sub("static") }

func sub(dir string) fs.FS {
	f, err := fs.Sub(files, dir)
	if err != nil {
		// Only reachable on a bad embed path (programmer error).
		panic(fmt.Sprintf("web: embed %s: %v", dir, err))
	}
	return f
}
