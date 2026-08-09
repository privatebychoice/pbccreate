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

func TestIdeaCreatePromoteFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/ideas", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Capture an idea with ICE factors.
	if rec := postForm(t, s, "/ideas", token, url.Values{
		"channel_id": {strconv.FormatInt(ch.ID, 10)},
		"title":      {"Best VPN 2026"},
		"impact":     {"9"},
		"confidence": {"8"},
		"effort":     {"2"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create idea = %d, want 303", rec.Code)
	}
	ideas, _ := store.ListIdeas(ctx, s.db)
	if len(ideas) != 1 {
		t.Fatalf("want 1 idea, got %d", len(ideas))
	}
	idea := ideas[0]

	// The list shows the computed score (9*8/2 = 36.0).
	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/ideas", nil))
	if !strings.Contains(listRec.Body.String(), "36.0") {
		t.Error("ideas list missing computed score 36.0")
	}

	// Promote it to a video content item.
	promo := postForm(t, s, "/ideas/"+strconv.FormatInt(idea.ID, 10)+"/promote", token, url.Values{
		"type": {"video"},
	})
	if promo.Code != http.StatusSeeOther {
		t.Fatalf("promote = %d, want 303", promo.Code)
	}
	loc := promo.Header().Get("Location")
	if !strings.HasPrefix(loc, "/content/") {
		t.Fatalf("promote redirect = %q, want /content/<id>", loc)
	}
	itemID, _ := strconv.ParseInt(strings.TrimPrefix(loc, "/content/"), 10, 64)

	// A content item now exists at status "idea" with the idea's title.
	item, err := store.GetContentItem(ctx, s.db, itemID)
	if err != nil || item.Title != "Best VPN 2026" || item.Status != "idea" {
		t.Fatalf("promoted content item wrong: %v (%+v)", err, item)
	}

	// The idea is marked promoted and linked.
	after, _ := store.GetIdea(ctx, s.db, idea.ID)
	if after.Status != "promoted" || after.PromotedContentItemID != itemID {
		t.Fatalf("idea not marked promoted: %+v", after)
	}

	// An invalid promote type is a 400 (no second content item created).
	bad := postForm(t, s, "/ideas/"+strconv.FormatInt(idea.ID, 10)+"/promote", token, url.Values{"type": {"bogus"}})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("promote bad type = %d, want 400", bad.Code)
	}
}
