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

func TestRetrospectiveSave(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// The section renders on the detail page.
	if !strings.Contains(getRec.Body.String(), "Retrospective") {
		t.Error("detail page missing retrospective section")
	}

	if rec := postForm(t, s, base+"/retrospective", token, url.Values{
		"what_worked":       {"strong hook"},
		"to_improve":        {"tighten the middle"},
		"performance_notes": {"10k views"},
		"reviewed_on":       {"2026-08-09"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("save retrospective = %d, want 303", rec.Code)
	}

	got, _ := store.GetRetrospective(ctx, s.db, item.ID)
	if got.WhatWorked != "strong hook" || got.PerformanceNotes != "10k views" || got.ReviewedOn != "2026-08-09" {
		t.Fatalf("retrospective not saved: %+v", got)
	}

	// Saved values render back into the form.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detail.Body.String(), "strong hook") {
		t.Error("saved retrospective not shown on detail page")
	}
}
