package server

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
	"go.privatebychoice.com/pbccreate/internal/thumbnail"
)

func TestThumbnailCreateSaveRenderFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Create -> redirects to the editor.
	createRec := postForm(t, s, base+"/thumbnails", token, url.Values{"name": {"Main"}})
	if createRec.Code != http.StatusSeeOther {
		t.Fatalf("create = %d, want 303", createRec.Code)
	}
	loc := createRec.Header().Get("Location")
	if !strings.HasPrefix(loc, base+"/thumbnails/") {
		t.Fatalf("redirect = %q, want editor URL", loc)
	}

	th, err := store.ListThumbnails(context.Background(), s.db, item.ID)
	if err != nil || len(th) != 1 {
		t.Fatalf("ListThumbnails: %v (len=%d)", err, len(th))
	}
	thumbBase := base + "/thumbnails/" + strconv.FormatInt(th[0].ID, 10)

	// Save a red background + title via the canvas_json payload (as the editor does).
	saveRec := postForm(t, s, thumbBase, token, url.Values{
		"canvas_json": {`{"background":"#ff0000","layers":[{"type":"text","text":"BIG NEWS","x":100,"y":300,"fontSize":120,"color":"#ffffff","bold":true}]}`},
	})
	if saveRec.Code != http.StatusSeeOther {
		t.Fatalf("save = %d, want 303", saveRec.Code)
	}

	// Canvas persisted with the new values.
	got, _ := store.GetThumbnail(context.Background(), s.db, th[0].ID, item.ID)
	canvas, err := thumbnail.Parse(got.CanvasJSON)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if canvas.Background != "#ff0000" || len(canvas.Layers) != 1 || canvas.Layers[0].Text != "BIG NEWS" {
		t.Fatalf("canvas not saved: %+v", canvas)
	}

	// Render endpoint returns a valid 1280x720 PNG.
	renderRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(renderRec, httptest.NewRequest(http.MethodGet, thumbBase+"/render.png", nil))
	if renderRec.Code != http.StatusOK {
		t.Fatalf("render = %d, want 200", renderRec.Code)
	}
	if ct := renderRec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	img, err := png.Decode(bytes.NewReader(renderRec.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if img.Bounds().Dx() != thumbnail.CanvasW || img.Bounds().Dy() != thumbnail.CanvasH {
		t.Errorf("size = %v, want %dx%d", img.Bounds(), thumbnail.CanvasW, thumbnail.CanvasH)
	}

	// Delete.
	delRec := postForm(t, s, thumbBase+"/delete", token, url.Values{})
	if delRec.Code != http.StatusSeeOther {
		t.Fatalf("delete = %d, want 303", delRec.Code)
	}
}

func TestThumbnailRenderNotFound(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	rec := httptest.NewRecorder()
	url := "/content/" + strconv.FormatInt(item.ID, 10) + "/thumbnails/999999/render.png"
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("render missing = %d, want 404", rec.Code)
	}
}

func TestThumbnailSaveRejectsBadJSON(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	th, _ := store.CreateThumbnail(context.Background(), s.db, item.ID, "T", "{}")
	thumbBase := base + "/thumbnails/" + strconv.FormatInt(th.ID, 10)

	rec := postForm(t, s, thumbBase, token, url.Values{"canvas_json": {"{not valid json"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad canvas = %d, want 400", rec.Code)
	}
}

func TestFontEndpoints(t *testing.T) {
	s := newTestServerWithDB(t)
	for _, path := range []string{"/static/fonts/go-regular.ttf", "/static/fonts/go-bold.ttf"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "font/ttf" {
			t.Errorf("%s content-type = %q, want font/ttf", path, ct)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s body empty", path)
		}
	}
}
