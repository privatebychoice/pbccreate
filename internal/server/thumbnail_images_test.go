package server

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
	"go.privatebychoice.com/pbccreate/internal/thumbnail"
)

// pngBytes builds a small solid-color PNG for upload tests.
func pngBytes(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// uploadImage posts a multipart image upload with the CSRF token/cookie.
func uploadImage(t *testing.T, s *Server, path, token string, data []byte, asBackground bool) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("csrf_token", token)
	if asBackground {
		_ = mw.WriteField("as_background", "on")
	}
	fw, err := mw.CreateFormFile("image", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestThumbnailImageUploadAndServe(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	th, _ := store.CreateThumbnail(context.Background(), s.db, item.ID, "Main", `{"background":"#000000"}`)
	thumbBase := base + "/thumbnails/" + strconv.FormatInt(th.ID, 10)

	// Upload a 300x200 red PNG as a normal image layer.
	up := uploadImage(t, s, thumbBase+"/images", token, pngBytes(t, 300, 200, color.RGBA{255, 0, 0, 255}), false)
	if up.Code != http.StatusSeeOther {
		t.Fatalf("upload = %d, want 303", up.Code)
	}

	// The canvas now has an image layer referencing the stored image.
	got, _ := store.GetThumbnail(context.Background(), s.db, th.ID, item.ID)
	canvas, err := thumbnail.Parse(got.CanvasJSON)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var imgLayer *thumbnail.Layer
	for i := range canvas.Layers {
		if canvas.Layers[i].Type == "image" {
			imgLayer = &canvas.Layers[i]
		}
	}
	if imgLayer == nil || imgLayer.ImageID == 0 {
		t.Fatalf("no image layer added: %+v", canvas.Layers)
	}

	// Serve the uploaded image (as PNG).
	serveRec := httptest.NewRecorder()
	serveURL := base + "/thumbnail-images/" + strconv.FormatInt(imgLayer.ImageID, 10)
	s.Handler().ServeHTTP(serveRec, httptest.NewRequest(http.MethodGet, serveURL, nil))
	if serveRec.Code != http.StatusOK {
		t.Fatalf("serve = %d, want 200", serveRec.Code)
	}
	if ct := serveRec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	if _, err := png.Decode(bytes.NewReader(serveRec.Body.Bytes())); err != nil {
		t.Errorf("served bytes not a PNG: %v", err)
	}

	// The rendered thumbnail composites the image (red pixel where placed).
	renderRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(renderRec, httptest.NewRequest(http.MethodGet, thumbBase+"/render.png", nil))
	rimg, err := png.Decode(bytes.NewReader(renderRec.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode render: %v", err)
	}
	r, g, b, _ := rimg.At(5, 5).RGBA() // layer placed at 0,0
	if r>>8 < 200 || g>>8 > 60 || b>>8 > 60 {
		t.Errorf("composited pixel = (%d,%d,%d), want red", r>>8, g>>8, b>>8)
	}
}

func TestThumbnailImageUploadBackgroundPrepends(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	th, _ := store.CreateThumbnail(context.Background(), s.db, item.ID,
		"Main", `{"background":"#000000","layers":[{"type":"text","text":"HI","x":10,"y":10,"fontSize":80,"color":"#fff"}]}`)
	thumbBase := base + "/thumbnails/" + strconv.FormatInt(th.ID, 10)

	up := uploadImage(t, s, thumbBase+"/images", token, pngBytes(t, 100, 100, color.RGBA{0, 0, 255, 255}), true)
	if up.Code != http.StatusSeeOther {
		t.Fatalf("upload = %d, want 303", up.Code)
	}

	got, _ := store.GetThumbnail(context.Background(), s.db, th.ID, item.ID)
	canvas, _ := thumbnail.Parse(got.CanvasJSON)
	if len(canvas.Layers) != 2 || canvas.Layers[0].Type != "image" {
		t.Fatalf("background image should be first layer: %+v", canvas.Layers)
	}
	if canvas.Layers[0].W != thumbnail.CanvasW || canvas.Layers[0].H != thumbnail.CanvasH {
		t.Errorf("background not full-canvas: %dx%d", canvas.Layers[0].W, canvas.Layers[0].H)
	}
}

func TestThumbnailImageUploadRejectsNonImage(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	th, _ := store.CreateThumbnail(context.Background(), s.db, item.ID, "Main", "{}")
	thumbBase := base + "/thumbnails/" + strconv.FormatInt(th.ID, 10)

	up := uploadImage(t, s, thumbBase+"/images", token, []byte("this is not an image"), false)
	if up.Code != http.StatusBadRequest {
		t.Fatalf("non-image upload = %d, want 400", up.Code)
	}
}

func TestCleanFilename(t *testing.T) {
	cases := map[string]string{
		"/etc/passwd":      "passwd",
		"  photo.png  ":    "photo.png",
		"":                 "image",
		"../../secret.jpg": "secret.jpg",
	}
	for in, want := range cases {
		if got := cleanFilename(in); got != want {
			t.Errorf("cleanFilename(%q) = %q, want %q", in, got, want)
		}
	}
	if len(cleanFilename(strings.Repeat("a", 300))) != 120 {
		t.Error("long filename not truncated to 120")
	}
}
