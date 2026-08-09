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

func TestFormatDefineAndSeed(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/formats", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Define a format → redirect to its detail page.
	rec := postForm(t, s, "/formats", token, url.Values{
		"channel_id":   {strconv.FormatInt(ch.ID, 10)},
		"name":         {"Tutorial"},
		"default_type": {"video"},
		"default_mode": {"single_cam"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create format = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	formatID, _ := strconv.ParseInt(strings.TrimPrefix(loc, "/formats/"), 10, 64)

	// Add two default outline segments.
	for _, seg := range []struct{ title, secs string }{{"Hook", "15"}, {"Body", "120"}} {
		if rec := postForm(t, s, loc+"/segments", token, url.Values{"title": {seg.title}, "target_seconds": {seg.secs}}); rec.Code != http.StatusSeeOther {
			t.Fatalf("add segment = %d, want 303", rec.Code)
		}
	}
	segs, _ := store.ListFormatSegments(ctx, s.db, formatID)
	if len(segs) != 2 {
		t.Fatalf("want 2 format segments, got %d", len(segs))
	}

	// Add a default shot.
	if rec := postForm(t, s, loc+"/shots", token, url.Values{"description": {"Wide establishing"}, "camera": {"A"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add format shot = %d, want 303", rec.Code)
	}
	if fshots, _ := store.ListFormatShots(ctx, s.db, formatID); len(fshots) != 1 {
		t.Fatalf("want 1 format shot, got %d", len(fshots))
	}

	// Seed a content item from the format.
	seed := postForm(t, s, loc+"/seed", token, url.Values{"title": {"My Tutorial"}})
	if seed.Code != http.StatusSeeOther {
		t.Fatalf("seed = %d, want 303", seed.Code)
	}
	itemLoc := seed.Header().Get("Location")
	if !strings.HasPrefix(itemLoc, "/content/") {
		t.Fatalf("seed redirect = %q, want /content/<id>", itemLoc)
	}
	itemID, _ := strconv.ParseInt(strings.TrimPrefix(itemLoc, "/content/"), 10, 64)

	item, err := store.GetContentItem(ctx, s.db, itemID)
	if err != nil || item.Title != "My Tutorial" || item.Type != "video" || item.Mode != "single_cam" {
		t.Fatalf("seeded item wrong: %v (%+v)", err, item)
	}
	outline, _ := store.ListOutlineSegments(ctx, s.db, itemID)
	if len(outline) != 2 || outline[0].Title != "Hook" {
		t.Fatalf("seeded outline wrong: %+v", outline)
	}
	shots, _ := store.ListShots(ctx, s.db, itemID)
	if len(shots) != 1 || shots[0].Description != "Wide establishing" {
		t.Fatalf("seeded shots wrong: %+v", shots)
	}

	// Seeding without a title is a 400.
	if bad := postForm(t, s, loc+"/seed", token, url.Values{"title": {"  "}}); bad.Code != http.StatusBadRequest {
		t.Fatalf("seed without title = %d, want 400", bad.Code)
	}
}
