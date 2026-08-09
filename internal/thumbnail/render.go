package thumbnail

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	fontOnce             sync.Once
	regularFace, boldTTF *opentype.Font
	fontErr              error
)

func loadFonts() {
	fontOnce.Do(func() {
		regularFace, fontErr = opentype.Parse(goregular.TTF)
		if fontErr != nil {
			return
		}
		boldTTF, fontErr = opentype.Parse(gobold.TTF)
	})
}

// Render draws the canvas to a fixed 1280x720 RGBA image. It is deterministic:
// the same canvas always yields the same pixels (SPEC §5.5, §6).
func Render(c Canvas) (image.Image, error) {
	loadFonts()
	if fontErr != nil {
		return nil, fmt.Errorf("load fonts: %w", fontErr)
	}

	img := image.NewRGBA(image.Rect(0, 0, CanvasW, CanvasH))
	bg := parseHexColor(c.Background, color.RGBA{16, 20, 24, 255})
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	for _, l := range c.Layers {
		if l.Type != "text" || strings.TrimSpace(l.Text) == "" {
			continue
		}
		if err := drawText(img, l); err != nil {
			return nil, err
		}
	}
	return img, nil
}

func drawText(img *image.RGBA, l Layer) error {
	size := clampInt(l.FontSize, 8, 400)
	ttf := regularFace
	if l.Bold {
		ttf = boldTTF
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size: float64(size), DPI: 72, Hinting: font.HintingFull,
	})
	if err != nil {
		return fmt.Errorf("new face: %w", err)
	}
	defer func() { _ = face.Close() }()

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(parseHexColor(l.Color, color.RGBA{255, 255, 255, 255})),
		Face: face,
	}
	m := face.Metrics()
	ascent := m.Ascent.Ceil()
	lineHeight := m.Height.Ceil()

	// Y is the top of the text block; add ascent to reach the first baseline.
	for i, line := range strings.Split(l.Text, "\n") {
		d.Dot = fixed.P(l.X, l.Y+ascent+i*lineHeight)
		d.DrawString(line)
	}
	return nil
}

// parseHexColor parses "#RRGGBB" (with or without leading #), returning def on
// any error.
func parseHexColor(s string, def color.RGBA) color.RGBA {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return def
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
