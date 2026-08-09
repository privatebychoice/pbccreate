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

func TestShotAddAndStatusFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add a shot.
	if rec := postForm(t, s, base+"/shots", token, url.Values{
		"description": {"Wide establishing shot"},
		"camera":      {"A"},
		"framing":     {"WS"},
		"status":      {"planned"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add shot = %d, want 303", rec.Code)
	}

	// Detail shows it.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detRec.Body.String(), "Wide establishing shot") {
		t.Error("shot not shown on detail")
	}

	// Update its status to "selected".
	shots, err := store.ListShots(context.Background(), s.db, item.ID)
	if err != nil || len(shots) != 1 {
		t.Fatalf("ListShots: %v (len=%d)", err, len(shots))
	}
	statusPath := base + "/shots/" + strconv.FormatInt(shots[0].ID, 10) + "/status"
	if rec := postForm(t, s, statusPath, token, url.Values{"status": {"selected"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("set status = %d, want 303", rec.Code)
	}
	shots, _ = store.ListShots(context.Background(), s.db, item.ID)
	if shots[0].Status != "selected" {
		t.Errorf("status = %q, want selected", shots[0].Status)
	}
}

func TestShotDeleteNotFound(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/shots/999999/delete", token, url.Values{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing shot = %d, want 404", rec.Code)
	}
}
