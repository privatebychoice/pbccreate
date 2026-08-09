package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/config"
	"go.privatebychoice.com/pbccreate/internal/store"
)

func newTestServerWithDB(t *testing.T) *Server {
	t.Helper()
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(ctx, t.TempDir()+"/test.db", log)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.Migrate(ctx, db, store.MigrationsFS(), log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s, err := New(&config.Config{Addr: "127.0.0.1:0"}, db, log)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestChannelsCreateFlow(t *testing.T) {
	s := newTestServerWithDB(t)

	// GET the form to obtain a CSRF token.
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/channels", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /channels = %d, want 200", getRec.Code)
	}
	token := getCSRFCookie(getRec.Result().Cookies())
	if token == "" {
		t.Fatal("no CSRF cookie on GET /channels")
	}

	// POST the new-channel form with a matching token and same origin.
	form := url.Values{
		"name":       {"The Untracked Life"},
		"kind":       {"youtube"},
		"csrf_token": {token},
	}
	req := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	postRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(postRec, req)

	if postRec.Code != http.StatusSeeOther {
		t.Fatalf("POST /channels = %d, want 303", postRec.Code)
	}
	if loc := postRec.Header().Get("Location"); loc != "/channels" {
		t.Errorf("redirect Location = %q, want /channels", loc)
	}

	// The channel now appears in the list.
	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/channels", nil))
	if !strings.Contains(listRec.Body.String(), "The Untracked Life") {
		t.Error("created channel not shown in list")
	}
}

func TestChannelsCreateValidationError(t *testing.T) {
	s := newTestServerWithDB(t)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/channels", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	form := url.Values{"name": {"   "}, "csrf_token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST invalid = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Channel name is required") {
		t.Error("validation message not rendered")
	}
}
