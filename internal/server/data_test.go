package server

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestProjectRootSettingAndHomeNudge(t *testing.T) {
	s := newTestServerWithDB(t)

	// Unset: the Data page shows "not set" and the home page shows the nudge.
	dataRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(dataRec, httptest.NewRequest(http.MethodGet, "/data", nil))
	if !strings.Contains(dataRec.Body.String(), "DaVinci project root") || !strings.Contains(dataRec.Body.String(), "not set") {
		t.Error("data page missing project-root section / not-set state")
	}
	token := getCSRFCookie(dataRec.Result().Cookies())

	homeRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(homeRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(homeRec.Body.String(), "DaVinci project root") {
		t.Error("home page missing the setup nudge when project root is unset")
	}

	// Save a project root.
	if rec := postForm(t, s, "/data/project-root", token, url.Values{"project_root": {"/Users/example/Video/Projects"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("save project root = %d, want 303", rec.Code)
	}
	if v, _ := store.GetSetting(context.Background(), s.db, store.SettingProjectRoot); v != "/Users/example/Video/Projects" {
		t.Fatalf("stored project root = %q", v)
	}

	// Now the Data page reflects the stored value and the home nudge is gone.
	dataRec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(dataRec2, httptest.NewRequest(http.MethodGet, "/data", nil))
	if !strings.Contains(dataRec2.Body.String(), "/Users/example/Video/Projects") {
		t.Error("data page does not show the saved project root")
	}
	homeRec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(homeRec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(homeRec2.Body.String(), "configure it on the Data page") {
		t.Error("home nudge should disappear once the project root is set")
	}
}

func TestBackupDownload(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	_, _ = store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Backup me")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/data/backup", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("backup = %d, want 200", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".db") {
		t.Errorf("content-disposition = %q, want attachment .db", cd)
	}
	// The body is a real SQLite database (magic header).
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("SQLite format 3\x00")) {
		t.Error("backup body is not a SQLite database")
	}
}

// uploadCSV posts a multipart CSV to path with the CSRF token/cookie.
func uploadCSV(t *testing.T, s *Server, path, token, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("csrf_token", token)
	fw, err := mw.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
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

func TestBulkImportContent(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	if _, err := store.CreateChannel(ctx, s.db, "TUL", "youtube"); err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/data", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	csv := strings.Join([]string{
		"channel,title,type,mode,status",
		"TUL,Best VPN 2026,video,single_cam,published", // ok
		"TUL,Quick tip,short,,scheduled",               // ok
		"Nope,Orphan,video,,",                          // skip: unknown channel
		"TUL,,video,,",                                 // skip: missing title
		"TUL,Bad status,video,,notastatus",             // created, but status skipped
	}, "\n")

	rec := uploadCSV(t, s, "/data/import", token, csv)
	if rec.Code != http.StatusOK {
		t.Fatalf("import = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Imported 3 item(s)") {
		t.Errorf("expected 3 imported; body: %s", body)
	}
	if !strings.Contains(body, "no channel named") || !strings.Contains(body, "channel and title are required") {
		t.Error("expected skip reasons for unknown channel and missing title")
	}

	items, _ := store.ListContentItems(ctx, s.db)
	if len(items) != 3 {
		t.Fatalf("want 3 content items created, got %d", len(items))
	}
	// The published row carried its status through.
	var publishedFound bool
	for _, it := range items {
		if it.Title == "Best VPN 2026" && it.Status == "published" {
			publishedFound = true
		}
	}
	if !publishedFound {
		t.Error("expected 'Best VPN 2026' imported at status published")
	}
}
