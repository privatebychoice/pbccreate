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

func TestSeriesPlanFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	a, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Ep1")
	b, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Ep2")

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/series", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Create a series → redirect to its detail page.
	rec := postForm(t, s, "/series", token, url.Values{
		"channel_id": {strconv.FormatInt(ch.ID, 10)},
		"name":       {"VPN Deep Dive"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create series = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	seriesID, _ := strconv.ParseInt(strings.TrimPrefix(loc, "/series/"), 10, 64)

	// Add both episodes.
	for _, it := range []store.ContentItem{a, b} {
		if rec := postForm(t, s, loc+"/episodes", token, url.Values{"content_item_id": {strconv.FormatInt(it.ID, 10)}}); rec.Code != http.StatusSeeOther {
			t.Fatalf("add episode = %d, want 303", rec.Code)
		}
	}
	eps, _ := store.ListEpisodes(ctx, s.db, seriesID)
	if len(eps) != 2 || eps[0].Title != "Ep1" {
		t.Fatalf("episodes wrong: %+v", eps)
	}

	// Move Ep2 up.
	if rec := postForm(t, s, loc+"/episodes/"+strconv.FormatInt(eps[1].ID, 10)+"/move", token, url.Values{"dir": {"up"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("move episode = %d, want 303", rec.Code)
	}
	eps, _ = store.ListEpisodes(ctx, s.db, seriesID)
	if eps[0].Title != "Ep2" {
		t.Fatalf("order after move wrong: %+v", eps)
	}

	// Arc notes save.
	if rec := postForm(t, s, loc+"/episodes/"+strconv.FormatInt(eps[0].ID, 10)+"/arc", token, url.Values{"arc_notes": {"cold open"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("arc = %d, want 303", rec.Code)
	}

	// Publish one → the list shows 1/2 progress.
	_ = store.UpdateContentItemStatus(ctx, s.db, a.ID, "published")
	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/series", nil))
	if !strings.Contains(listRec.Body.String(), "1 / 2 published") {
		t.Error("series list missing 1 / 2 progress")
	}

	// Remove an episode; the content item survives.
	if rec := postForm(t, s, loc+"/episodes/"+strconv.FormatInt(eps[0].ID, 10)+"/remove", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("remove = %d, want 303", rec.Code)
	}
	if eps, _ := store.ListEpisodes(ctx, s.db, seriesID); len(eps) != 1 {
		t.Fatalf("want 1 episode after remove, got %d", len(eps))
	}
	if _, err := store.GetContentItem(ctx, s.db, b.ID); err != nil {
		t.Errorf("content item should survive: %v", err)
	}
}
