// Package thumbnail is the server-side render authority for YouTube thumbnails:
// it turns a canvas model (stored as JSON) into a deterministic PNG using the
// standard library and golang.org/x/image (see docs/SPEC.md §5.5, §6). The
// browser provides a WYSIWYG editing surface; this package is the source of
// truth for exports.
package thumbnail

import (
	"encoding/json"
	"fmt"
)

// Canvas dimensions are fixed to the YouTube thumbnail size (SPEC §5.5).
const (
	CanvasW = 1280
	CanvasH = 720
)

// Canvas is the thumbnail model: a background plus ordered layers (drawn back to
// front).
type Canvas struct {
	Background string  `json:"background"`
	Layers     []Layer `json:"layers"`
}

// Layer is a single element. v1 supports text layers; image/shape layers arrive
// in later slices.
type Layer struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	FontSize int    `json:"fontSize,omitempty"`
	Color    string `json:"color,omitempty"`
	Bold     bool   `json:"bold,omitempty"`
}

// DefaultCanvas returns a starter canvas: a dark background with one title.
func DefaultCanvas() Canvas {
	return Canvas{
		Background: "#101418",
		Layers: []Layer{{
			Type: "text", Text: "Your Title", X: 80, Y: 280,
			FontSize: 120, Color: "#ffffff", Bold: true,
		}},
	}
}

// Parse decodes a canvas from stored JSON, falling back to a default background
// when empty.
func Parse(data string) (Canvas, error) {
	if data == "" {
		return Canvas{Background: "#101418"}, nil
	}
	var c Canvas
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		return Canvas{}, fmt.Errorf("parse canvas: %w", err)
	}
	if c.Background == "" {
		c.Background = "#101418"
	}
	return c, nil
}

// JSON encodes the canvas for storage.
func (c Canvas) JSON() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encode canvas: %w", err)
	}
	return string(b), nil
}
