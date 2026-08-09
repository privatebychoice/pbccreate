package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/media"
	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestMediaPreviewNotFoundWithoutPreview(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	// Catalogue an asset with no preview (path need not exist for this check).
	a, err := store.AddMediaAsset(context.Background(), s.db, store.MediaAsset{
		ContentItemID: item.ID, Path: "/nope/clip.mp4", Kind: "video",
	})
	if err != nil {
		t.Fatalf("AddMediaAsset: %v", err)
	}
	url := "/content/" + strconv.FormatInt(item.ID, 10) + "/media/" + strconv.FormatInt(a.ID, 10) + "/preview"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("preview without file = %d, want 404", rec.Code)
	}
}

// TestMediaPreviewGenerationFlow generates a real clip, adds it (preview-on-add),
// then fetches the preview image. Skipped unless ffmpeg is installed.
func TestMediaPreviewGenerationFlow(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; skipping")
	}
	if !media.ThumbAvailable("") {
		t.Skip("ffmpeg not available; skipping")
	}

	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	// Scope a media root and generate a real clip in it.
	root := t.TempDir()
	s.cfg.MediaRoots = []string{root}
	src := root + "/clip.mp4"
	gen := exec.Command("ffmpeg", "-nostdin", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=640x360:rate=30",
		"-pix_fmt", "yuv420p", src)
	if b, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate: %v\n%s", err, b)
	}

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add -> preview generated on add.
	if rec := postForm(t, s, base+"/media", token, url.Values{"path": {src}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add media = %d, want 303", rec.Code)
	}

	assets, _ := store.ListMediaAssets(context.Background(), s.db, item.ID)
	if len(assets) != 1 || assets[0].PreviewPath == "" {
		t.Fatalf("preview not recorded on add: %+v", assets)
	}

	// Fetch the preview image.
	previewURL := base + "/media/" + strconv.FormatInt(assets[0].ID, 10) + "/preview"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, previewURL, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get preview = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/jpeg") {
		t.Errorf("content-type = %q, want image/jpeg", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("empty preview body")
	}
}
