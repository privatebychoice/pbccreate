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

func TestTitleCandidatesAndSwipeFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Working")
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add two candidates.
	for _, txt := range []string{"How I built X", "The truth about X"} {
		if rec := postForm(t, s, base+"/titles", token, url.Values{"text": {txt}}); rec.Code != http.StatusSeeOther {
			t.Fatalf("add candidate = %d, want 303", rec.Code)
		}
	}
	cands, _ := store.ListTitleCandidates(ctx, s.db, item.ID)
	if len(cands) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(cands))
	}

	// Choose one.
	chosen := cands[1]
	if rec := postForm(t, s, base+"/titles/"+strconv.FormatInt(chosen.ID, 10)+"/choose", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("choose = %d, want 303", rec.Code)
	}
	after, _ := store.ListTitleCandidates(ctx, s.db, item.ID)
	if !after[0].Chosen || after[0].ID != chosen.ID {
		t.Fatalf("chosen not applied: %+v", after)
	}

	// The detail page shows the candidate.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detail.Body.String(), "The truth about X") {
		t.Error("detail missing title candidate")
	}

	// Swipe file: add a pattern (channel-scoped) and confirm it shows.
	if rec := postForm(t, s, base+"/swipe", token, url.Values{"pattern": {"How I X in Y"}, "note": {"worked"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add swipe = %d, want 303", rec.Code)
	}
	sw, _ := store.ListSwipe(ctx, s.db, ch.ID)
	if len(sw) != 1 {
		t.Fatalf("want 1 swipe, got %d", len(sw))
	}

	// Delete the chosen candidate.
	if rec := postForm(t, s, base+"/titles/"+strconv.FormatInt(chosen.ID, 10)+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete candidate = %d, want 303", rec.Code)
	}
	if c, _ := store.ListTitleCandidates(ctx, s.db, item.ID); len(c) != 1 {
		t.Errorf("want 1 candidate after delete, got %d", len(c))
	}
}
