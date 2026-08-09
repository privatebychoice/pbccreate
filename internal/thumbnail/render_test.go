package thumbnail

import (
	"image"
	"image/color"
	"testing"
)

func TestRenderSizeAndBackground(t *testing.T) {
	img, err := Render(Canvas{Background: "#ff0000"}, nil)
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
	}, nil)
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

func TestRenderImageLayer(t *testing.T) {
	// A 2x2 green source image, drawn scaled into a black canvas via a resolver.
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	green := color.RGBA{0, 255, 0, 255}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			src.Set(x, y, green)
		}
	}
	resolve := func(id int64) image.Image {
		if id == 7 {
			return src
		}
		return nil
	}

	img, err := Render(Canvas{
		Background: "#000000",
		Layers:     []Layer{{Type: "image", ImageID: 7, X: 100, Y: 100, W: 200, H: 200}},
	}, resolve)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	r, g, b, _ := img.At(150, 150).RGBA()
	if g>>8 < 200 || r>>8 > 60 || b>>8 > 60 {
		t.Errorf("image-layer pixel = (%d,%d,%d), want green", r>>8, g>>8, b>>8)
	}
	// Outside the layer stays black.
	r2, g2, b2, _ := img.At(10, 10).RGBA()
	if r2>>8 > 10 || g2>>8 > 10 || b2>>8 > 10 {
		t.Errorf("outside pixel = (%d,%d,%d), want black", r2>>8, g2>>8, b2>>8)
	}
}

func TestFitDownscales(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4000, 2000))
	out := Fit(src, 1920)
	if out.Bounds().Dx() != 1920 || out.Bounds().Dy() != 960 {
		t.Errorf("Fit size = %v, want 1920x960", out.Bounds())
	}
	// Already small: unchanged.
	small := image.NewRGBA(image.Rect(0, 0, 100, 50))
	if Fit(small, 1920) != small {
		t.Error("small image should be returned unchanged")
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
