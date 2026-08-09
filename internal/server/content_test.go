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

func TestContentCreateFlow(t *testing.T) {
	s := newTestServerWithDB(t)

	// Need a channel first.
	ch, err := store.CreateChannel(context.Background(), s.db, "TUL", "youtube")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	// GET the board to obtain a CSRF token.
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/content", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /content = %d, want 200", getRec.Code)
	}
	token := getCSRFCookie(getRec.Result().Cookies())
	if token == "" {
		t.Fatal("no CSRF cookie on GET /content")
	}

	// POST a new content item.
	form := url.Values{
		"channel_id": {strconv.FormatInt(ch.ID, 10)},
		"type":       {"video"},
		"mode":       {"single_cam"},
		"title":      {"Deband Your Footage"},
		"csrf_token": {token},
	}
	req := httptest.NewRequest(http.MethodPost, "/content", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	postRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(postRec, req)

	if postRec.Code != http.StatusSeeOther {
		t.Fatalf("POST /content = %d, want 303", postRec.Code)
	}

	// It appears on the board.
	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/content", nil))
	body := listRec.Body.String()
	if !strings.Contains(body, "Deband Your Footage") {
		t.Error("created item not shown on board")
	}
	// New items start in the "idea" column.
	if !strings.Contains(body, "Idea") {
		t.Error("board missing Idea column heading")
	}
}

func TestContentBoardPromptsForChannel(t *testing.T) {
	s := newTestServerWithDB(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Create a") {
		t.Error("expected prompt to create a channel when none exist")
	}
}
