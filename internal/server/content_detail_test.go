package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func seedItem(t *testing.T, s *Server) store.ContentItem {
	t.Helper()
	ctx := context.Background()
	ch, err := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	item, err := store.CreateContentItem(ctx, s.db, ch.ID, "video", "single_cam", "Pipeline Item")
	if err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}
	return item
}

func TestContentDetailView(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/"+strconv.FormatInt(item.ID, 10), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pipeline Item") {
		t.Error("detail missing title")
	}
	if !strings.Contains(body, "Update status") {
		t.Error("detail missing status form")
	}
}

// TestContentDetailTabTagging guards the data-tab attributes app.js relies on to
// build the tabbed editor. If the template loses them the tabs silently break.
func TestContentDetailTabTagging(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/"+strconv.FormatInt(item.ID, 10), nil))
	body := rec.Body.String()

	for _, tab := range []string{"plan", "media", "metadata", "rights", "release"} {
		if !strings.Contains(body, `data-tab="`+tab+`"`) {
			t.Errorf("rendered detail missing sections tagged data-tab=%q", tab)
		}
	}
}

func TestContentDetailNotFound(t *testing.T) {
	s := newTestServerWithDB(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/999999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestContentStatusUpdateFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)

	// Obtain a CSRF token from the board.
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/content", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	form := url.Values{
		"status":     {"scripting"},
		"return_to":  {"/content"},
		"csrf_token": {token},
	}
	req := httptest.NewRequest(http.MethodPost, "/content/"+strconv.FormatInt(item.ID, 10)+"/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/content" {
		t.Errorf("Location = %q, want /content", loc)
	}

	got, err := store.GetContentItem(context.Background(), s.db, item.ID)
	if err != nil {
		t.Fatalf("GetContentItem: %v", err)
	}
	if got.Status != "scripting" {
		t.Errorf("status = %q, want scripting", got.Status)
	}
}

func TestContentStatusOpenRedirectBlocked(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/content", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	form := url.Values{
		"status":     {"editing"},
		"return_to":  {"https://evil.example/phish"},
		"csrf_token": {token},
	}
	req := httptest.NewRequest(http.MethodPost, "/content/"+strconv.FormatInt(item.ID, 10)+"/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); loc != "/content" {
		t.Errorf("Location = %q, want /content (open redirect blocked)", loc)
	}
}
