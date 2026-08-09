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

// postForm issues a same-origin POST with the given CSRF token and returns the
// recorder.
func postForm(t *testing.T, s *Server, path, token string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	form.Set("csrf_token", token)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestOutlineAddDeleteFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add a segment.
	if rec := postForm(t, s, base+"/outline", token, url.Values{
		"title":          {"Hook"},
		"notes":          {"open strong"},
		"target_seconds": {"15"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add segment = %d, want 303", rec.Code)
	}

	// Detail shows it plus the total.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	body := detRec.Body.String()
	if !strings.Contains(body, "Hook") {
		t.Error("segment not shown on detail")
	}
	if !strings.Contains(body, "Total target: 15s") {
		t.Error("outline total not shown")
	}

	// Fetch its id from the store, then delete it.
	segs, err := store.ListOutlineSegments(context.Background(), s.db, item.ID)
	if err != nil || len(segs) != 1 {
		t.Fatalf("ListOutlineSegments: %v (len=%d)", err, len(segs))
	}
	delPath := base + "/outline/" + strconv.FormatInt(segs[0].ID, 10) + "/delete"
	if rec := postForm(t, s, delPath, token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete segment = %d, want 303", rec.Code)
	}

	segs, _ = store.ListOutlineSegments(context.Background(), s.db, item.ID)
	if len(segs) != 0 {
		t.Errorf("segment not deleted: %d remain", len(segs))
	}
}

func TestOutlineMoveFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	postForm(t, s, base+"/outline", token, url.Values{"title": {"A"}, "target_seconds": {"0"}})
	postForm(t, s, base+"/outline", token, url.Values{"title": {"B"}, "target_seconds": {"0"}})

	segs, _ := store.ListOutlineSegments(context.Background(), s.db, item.ID)
	if len(segs) != 2 {
		t.Fatalf("want 2 segments, got %d", len(segs))
	}

	// Move B up; order becomes B, A.
	movePath := base + "/outline/" + strconv.FormatInt(segs[1].ID, 10) + "/move"
	if rec := postForm(t, s, movePath, token, url.Values{"dir": {"up"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("move = %d, want 303", rec.Code)
	}
	segs, _ = store.ListOutlineSegments(context.Background(), s.db, item.ID)
	if segs[0].Title != "B" || segs[1].Title != "A" {
		t.Errorf("order = %q,%q want B,A", segs[0].Title, segs[1].Title)
	}
}
