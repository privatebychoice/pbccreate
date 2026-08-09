package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestMediaAddVerifyFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	// Create a real file inside a temp media root and scope the server to it.
	root := t.TempDir()
	s.cfg.MediaRoots = []string{root}
	file := filepath.Join(root, "aroll.mov")
	if err := os.WriteFile(file, []byte("fake video bytes"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add the media file (kind auto-detected from .mov).
	if rec := postForm(t, s, base+"/media", token, url.Values{
		"path":   {file},
		"status": {"recorded"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add media = %d, want 303", rec.Code)
	}

	assets, err := store.ListMediaAssets(context.Background(), s.db, item.ID)
	if err != nil || len(assets) != 1 {
		t.Fatalf("ListMediaAssets: %v (len=%d)", err, len(assets))
	}
	if assets[0].Kind != "video" || !assets[0].Present || assets[0].SizeBytes == 0 {
		t.Errorf("unexpected asset: kind=%q present=%v size=%d", assets[0].Kind, assets[0].Present, assets[0].SizeBytes)
	}

	// Delete the file on disk, then verify -> should be flagged missing.
	if err := os.Remove(file); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	if rec := postForm(t, s, base+"/media/verify", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("verify = %d, want 303", rec.Code)
	}
	assets, _ = store.ListMediaAssets(context.Background(), s.db, item.ID)
	if assets[0].Present {
		t.Error("asset should be flagged missing after file removal")
	}

	// The detail page shows the "missing" badge.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detRec.Body.String(), "missing") {
		t.Error("detail page missing the 'missing' indicator")
	}
}

func TestMediaAddRejectsPathOutsideRoots(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)
	s.cfg.MediaRoots = []string{t.TempDir()}

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/media", token, url.Values{"path": {"/etc/passwd"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("add outside root = %d, want 400", rec.Code)
	}
	assets, _ := store.ListMediaAssets(context.Background(), s.db, item.ID)
	if len(assets) != 0 {
		t.Errorf("rejected path was catalogued: %d assets", len(assets))
	}
}
