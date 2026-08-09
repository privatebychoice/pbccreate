package server

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

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
