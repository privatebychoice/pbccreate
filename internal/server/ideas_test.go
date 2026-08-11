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

func TestIdeaPillarLinkAndCarryOnPromote(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	pillar, _ := store.CreatePillar(ctx, s.db, ch.ID, "Privacy", "theme")
	idea, _ := store.CreateIdea(ctx, s.db, store.Idea{ChannelID: ch.ID, Title: "VPN guide"})
	ideaPath := "/ideas/" + strconv.FormatInt(idea.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, ideaPath, nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	// The detail page offers the channel's pillar in a select.
	if !strings.Contains(getRec.Body.String(), `name="pillar_id"`) || !strings.Contains(getRec.Body.String(), "Privacy") {
		t.Error("idea detail missing pillar select / option")
	}

	// Assign the pillar via the edit form.
	if rec := postForm(t, s, ideaPath, token, url.Values{
		"title":     {"VPN guide"},
		"pillar_id": {strconv.FormatInt(pillar.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update idea = %d, want 303", rec.Code)
	}
	if got, _ := store.GetIdea(ctx, s.db, idea.ID); got.PillarID != pillar.ID {
		t.Fatalf("pillar not linked: %+v", got)
	}

	// A pillar from another channel is a 400.
	other, _ := store.CreateChannel(ctx, s.db, "PBC", "youtube")
	foreign, _ := store.CreatePillar(ctx, s.db, other.ID, "Foreign", "")
	if rec := postForm(t, s, ideaPath, token, url.Values{
		"title":     {"VPN guide"},
		"pillar_id": {strconv.FormatInt(foreign.ID, 10)},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign pillar = %d, want 400", rec.Code)
	}

	// Promote carries the pillar onto the new content item.
	promo := postForm(t, s, ideaPath+"/promote", token, url.Values{"type": {"video"}})
	if promo.Code != http.StatusSeeOther {
		t.Fatalf("promote = %d, want 303", promo.Code)
	}
	itemID, _ := strconv.ParseInt(strings.TrimPrefix(promo.Header().Get("Location"), "/content/"), 10, 64)
	pillars, _ := store.ListPillarsForItem(ctx, s.db, itemID)
	if len(pillars) != 1 || pillars[0].ID != pillar.ID {
		t.Fatalf("promoted item pillars = %+v, want [Privacy]", pillars)
	}
}

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
