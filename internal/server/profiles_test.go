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

func TestShootProfileDefineAndAssign(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/profiles", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Define a gear profile and a location profile via the management surface.
	if rec := postForm(t, s, "/profiles", token, url.Values{
		"channel_id": {strconv.FormatInt(ch.ID, 10)},
		"kind":       {"gear"},
		"name":       {"A-cam kit"},
		"details":    {"camera + 24-70"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gear = %d, want 303", rec.Code)
	}
	if rec := postForm(t, s, "/profiles", token, url.Values{
		"channel_id": {strconv.FormatInt(ch.ID, 10)},
		"kind":       {"location"},
		"name":       {"Home studio"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create location = %d, want 303", rec.Code)
	}
	if g, _ := store.ListShootProfiles(ctx, s.db, "gear"); len(g) != 1 {
		t.Fatalf("want 1 gear profile, got %d", len(g))
	}

	// Assign gear to the item (get-or-create by name; case-insensitive → no dup).
	if rec := postForm(t, s, base+"/profiles", token, url.Values{"kind": {"gear"}, "name": {"a-cam kit"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("assign gear = %d, want 303", rec.Code)
	}
	if g, _ := store.ListProfilesForItem(ctx, s.db, item.ID, "gear"); len(g) != 1 {
		t.Fatalf("gear not assigned: %+v", g)
	}
	if all, _ := store.ListShootProfiles(ctx, s.db, "gear"); len(all) != 1 {
		t.Fatalf("assign created a duplicate gear profile: %d", len(all))
	}

	// Assign a location.
	if rec := postForm(t, s, base+"/profiles", token, url.Values{"kind": {"location"}, "name": {"Home studio"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("assign location = %d, want 303", rec.Code)
	}
	locs, _ := store.ListProfilesForItem(ctx, s.db, item.ID, "location")
	if len(locs) != 1 {
		t.Fatalf("location not assigned: %+v", locs)
	}

	// The item page shows both under their sections.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, base, nil))
	body := detail.Body.String()
	if !strings.Contains(body, "A-cam kit") || !strings.Contains(body, "Home studio") {
		t.Error("item detail missing gear/location chips")
	}

	// Remove the gear assignment; the profile remains defined.
	gear, _ := store.ListProfilesForItem(ctx, s.db, item.ID, "gear")
	if rec := postForm(t, s, base+"/profiles/"+strconv.FormatInt(gear[0].ID, 10)+"/remove", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("remove gear = %d, want 303", rec.Code)
	}
	if g, _ := store.ListProfilesForItem(ctx, s.db, item.ID, "gear"); len(g) != 0 {
		t.Errorf("gear not unassigned: %+v", g)
	}
	if _, err := store.GetShootProfile(ctx, s.db, gear[0].ID); err != nil {
		t.Errorf("gear profile should remain after unassign: %v", err)
	}
}
