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

func TestPillarDefineAssignAndCoverage(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/pillars", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Define a pillar via the management surface.
	if rec := postForm(t, s, "/pillars", token, url.Values{
		"channel_id":  {strconv.FormatInt(ch.ID, 10)},
		"name":        {"Privacy"},
		"description": {"core theme"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create pillar = %d, want 303", rec.Code)
	}

	// Coverage view shows it with 0 items.
	covRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(covRec, httptest.NewRequest(http.MethodGet, "/pillars", nil))
	if !strings.Contains(covRec.Body.String(), "Privacy") {
		t.Error("coverage page missing Privacy")
	}

	// Assign to the item (get-or-create by name, case-insensitive → same pillar).
	if rec := postForm(t, s, base+"/pillars", token, url.Values{"name": {"privacy"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("assign pillar = %d, want 303", rec.Code)
	}
	forItem, _ := store.ListPillarsForItem(ctx, s.db, item.ID)
	if len(forItem) != 1 || forItem[0].Name != "Privacy" {
		t.Fatalf("pillar not assigned (or duplicated): %+v", forItem)
	}
	pillarID := forItem[0].ID

	// Only one pillar exists in the channel (no duplicate from the differing case).
	chPillars, _ := store.ListPillarsForChannel(ctx, s.db, ch.ID)
	if len(chPillars) != 1 {
		t.Fatalf("want 1 channel pillar, got %d", len(chPillars))
	}

	// Coverage now reports 1 item.
	cov, _ := store.ListPillarCoverage(ctx, s.db)
	if len(cov) != 1 || cov[0].ItemCount != 1 {
		t.Fatalf("coverage wrong: %+v", cov)
	}

	// Remove the assignment.
	if rec := postForm(t, s, base+"/pillars/"+strconv.FormatInt(pillarID, 10)+"/remove", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("remove pillar = %d, want 303", rec.Code)
	}
	if forItem, _ := store.ListPillarsForItem(ctx, s.db, item.ID); len(forItem) != 0 {
		t.Errorf("pillar not unassigned: %+v", forItem)
	}
	// The pillar itself still exists (unassign leaves it defined).
	if _, err := store.GetPillar(ctx, s.db, pillarID); err != nil {
		t.Errorf("pillar should remain after unassign: %v", err)
	}
}
