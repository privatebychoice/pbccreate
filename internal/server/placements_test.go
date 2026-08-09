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

func TestPlacementAndDeliverableFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	sp, _ := store.CreateSponsor(ctx, s.db, "Acme", "", "")
	camp, _ := store.CreateCampaign(ctx, s.db, store.Campaign{SponsorID: sp.ID, Name: "Spring"})

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Attach the campaign.
	att := postForm(t, s, base+"/placements", token, url.Values{
		"campaign_id": {strconv.FormatInt(camp.ID, 10)},
		"deadline":    {"2026-03-15"},
	})
	if att.Code != http.StatusSeeOther {
		t.Fatalf("attach = %d, want 303", att.Code)
	}
	placements, _ := store.ListPlacementsForItem(ctx, s.db, item.ID)
	if len(placements) != 1 {
		t.Fatalf("want 1 placement, got %d", len(placements))
	}
	pid := placements[0].ID

	// Detail shows the placement.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detRec.Body.String(), "Acme — Spring") {
		t.Error("detail missing placement")
	}

	// Add a deliverable, then toggle it done.
	pBase := base + "/placements/" + strconv.FormatInt(pid, 10)
	if rec := postForm(t, s, pBase+"/deliverables", token, url.Values{"description": {"60s read"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add deliverable = %d, want 303", rec.Code)
	}
	ds, _ := store.ListDeliverables(ctx, s.db, pid)
	if len(ds) != 1 {
		t.Fatalf("want 1 deliverable, got %d", len(ds))
	}
	if rec := postForm(t, s, pBase+"/deliverables/"+strconv.FormatInt(ds[0].ID, 10)+"/toggle", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("toggle = %d, want 303", rec.Code)
	}
	ds, _ = store.ListDeliverables(ctx, s.db, pid)
	if !ds[0].Done {
		t.Error("deliverable should be done after toggle")
	}

	// Detach.
	if rec := postForm(t, s, pBase+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("detach = %d, want 303", rec.Code)
	}
	placements, _ = store.ListPlacementsForItem(ctx, s.db, item.ID)
	if len(placements) != 0 {
		t.Errorf("placement not detached: %d remain", len(placements))
	}
}

func TestPlacementPromptsWhenNoCampaigns(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/"+strconv.FormatInt(item.ID, 10), nil))
	if !strings.Contains(rec.Body.String(), "Create a") {
		t.Error("expected prompt to create a sponsor campaign when none exist")
	}
}
