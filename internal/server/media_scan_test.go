package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestMediaScanImportAndDedup(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	root := t.TempDir()
	s.cfg.MediaRoots = []string{root}
	writeFile(t, filepath.Join(root, "a.mp4"))
	writeFile(t, filepath.Join(root, "b.png"))
	writeFile(t, filepath.Join(root, "sub", "c.wav"))
	writeFile(t, filepath.Join(root, "notes.txt")) // skipped (non-media)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// First scan imports the 3 media files.
	rec := postForm(t, s, base+"/media/scan", token, url.Values{"dir": {root}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("scan = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != base+"?added=3&skipped=0" {
		t.Errorf("Location = %q, want added=3&skipped=0", loc)
	}
	assets, _ := store.ListMediaAssets(context.Background(), s.db, item.ID)
	if len(assets) != 3 {
		t.Fatalf("cataloged %d, want 3", len(assets))
	}

	// Second scan finds the same files and skips them all.
	rec = postForm(t, s, base+"/media/scan", token, url.Values{"dir": {root}})
	if loc := rec.Header().Get("Location"); loc != base+"?added=0&skipped=3" {
		t.Errorf("re-scan Location = %q, want added=0&skipped=3", loc)
	}
	assets, _ = store.ListMediaAssets(context.Background(), s.db, item.ID)
	if len(assets) != 3 {
		t.Errorf("after re-scan cataloged %d, want 3 (no dupes)", len(assets))
	}
}

func TestMediaScanRejectsOutsideRoot(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)
	s.cfg.MediaRoots = []string{t.TempDir()}

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/media/scan", token, url.Values{"dir": {"/etc"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("scan outside root = %d, want 400", rec.Code)
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
}
