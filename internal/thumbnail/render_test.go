package thumbnail

import (
	"image/color"
	"testing"
)

func TestRenderSizeAndBackground(t *testing.T) {
	img, err := Render(Canvas{Background: "#ff0000"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != CanvasW || b.Dy() != CanvasH {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), CanvasW, CanvasH)
	}
	r, g, bl, _ := img.At(10, 10).RGBA()
	if r>>8 != 255 || g>>8 != 0 || bl>>8 != 0 {
		t.Errorf("bg pixel = (%d,%d,%d), want red", r>>8, g>>8, bl>>8)
	}
}

func TestRenderTextDrawsPixels(t *testing.T) {
	// Black background, white text: expect some near-white pixels from the text.
	img, err := Render(Canvas{
		Background: "#000000",
		Layers: []Layer{{
			Type: "text", Text: "HELLO", X: 100, Y: 300, FontSize: 120, Color: "#ffffff",
		}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	white := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 > 200 && g>>8 > 200 && bl>>8 > 200 {
				white++
			}
		}
	}
	if white == 0 {
		t.Error("expected text pixels, found none")
	}
}

func TestParseHexColor(t *testing.T) {
	def := color.RGBA{1, 2, 3, 255}
	if got := parseHexColor("#00ff80", def); got != (color.RGBA{0, 255, 128, 255}) {
		t.Errorf("got %v", got)
	}
	if got := parseHexColor("bad", def); got != def {
		t.Errorf("invalid should fall back to def, got %v", got)
	}
}

func TestCanvasJSONRoundTrip(t *testing.T) {
	c := DefaultCanvas()
	s, err := c.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	got, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Background != c.Background || len(got.Layers) != len(c.Layers) || got.Layers[0].Text != "Your Title" {
		t.Errorf("round trip mismatch: %+v", got)
	}
}
