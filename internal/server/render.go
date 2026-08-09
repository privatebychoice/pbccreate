package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"unicode"
)

// funcMap holds template helpers available to every page.
var funcMap = template.FuncMap{
	"humanize": humanize,
	"duration": formatDuration,
	"filesize": formatFileSize,
}

// formatFileSize renders a byte count as a human-readable size, or "—" if unknown.
func formatFileSize(bytes int64) string {
	if bytes <= 0 {
		return "—"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration renders a whole-second count as "Ns" or "Mm SSs".
func formatDuration(sec int) string {
	if sec < 0 {
		sec = 0
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	return fmt.Sprintf("%dm %02ds", sec/60, sec%60)
}

// humanize turns a snake_case identifier into a display label, e.g.
// "single_cam" -> "Single cam".
func humanize(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", " ")
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// templates holds one parsed template set per page, each combining the shared
// base layout with that page's "content" block. Rendering per-page sets (rather
// than one global set) keeps pages from colliding on the "content" define.
type templates struct {
	pages map[string]*template.Template
}

const baseLayout = "base.html.tmpl"

// parseTemplates builds a set for every "*.html.tmpl" page except the base
// layout, cloning the base into each.
func parseTemplates(fsys fs.FS) (*templates, error) {
	base, err := template.New(baseLayout).Funcs(funcMap).ParseFS(fsys, baseLayout)
	if err != nil {
		return nil, fmt.Errorf("parse base layout: %w", err)
	}

	names, err := fs.Glob(fsys, "*.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}

	set := make(map[string]*template.Template)
	for _, name := range names {
		if name == baseLayout {
			continue
		}
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone base for %s: %w", name, err)
		}
		page, err := clone.ParseFS(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("parse page %s: %w", name, err)
		}
		set[name] = page
	}
	return &templates{pages: set}, nil
}

// render writes the named page with the given HTTP status. It buffers first so a
// template error yields a clean 500 rather than a half-written response.
func (t *templates) render(w http.ResponseWriter, status int, page string, data any) error {
	tmpl, ok := t.pages[page]
	if !ok {
		return fmt.Errorf("unknown page %q", page)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, baseLayout, data); err != nil {
		return fmt.Errorf("execute %s: %w", page, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := buf.WriteTo(w)
	return err
}
