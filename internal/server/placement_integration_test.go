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

// attachCampaign wires up a sponsor+campaign+placement for the given item and
// returns the placement.
func attachCampaign(t *testing.T, s *Server, itemID int64, campaign store.Campaign) store.Placement {
	t.Helper()
	ctx := context.Background()
	sp, err := store.CreateSponsor(ctx, s.db, "Acme VPN", "", "")
	if err != nil {
		t.Fatalf("CreateSponsor: %v", err)
	}
	campaign.SponsorID = sp.ID
	c, err := store.CreateCampaign(ctx, s.db, campaign)
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	p, err := store.CreatePlacement(ctx, s.db, c.ID, itemID, "")
	if err != nil {
		t.Fatalf("CreatePlacement: %v", err)
	}
	return p
}

func TestDescriptionSponsorBlurbFromPlacements(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	attachCampaign(t, s, item.ID, store.Campaign{
		Name:          "Spring",
		TalkingPoints: "Acme keeps you private.",
		PromoCode:     "TUL10",
		TrackingLink:  "https://acme.example/tul",
	})

	base := "/content/" + strconv.FormatInt(item.ID, 10)
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/description/sponsor", token, url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("fill sponsor = %d, want 303", rec.Code)
	}

	d, err := store.GetDescription(context.Background(), s.db, item.ID)
	if err != nil {
		t.Fatalf("GetDescription: %v", err)
	}
	for _, want := range []string{"Sponsored by Acme VPN.", "Acme keeps you private.", "Use code TUL10: https://acme.example/tul"} {
		if !strings.Contains(d.Sponsor, want) {
			t.Errorf("sponsor blurb missing %q; got:\n%s", want, d.Sponsor)
		}
	}
}

func TestBoardShowsSponsoredBadge(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)

	// Before any placement: no badge.
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content", nil))
	if strings.Contains(rec.Body.String(), "badge-sponsor") {
		t.Error("board shows sponsored badge before any placement")
	}

	attachCampaign(t, s, item.ID, store.Campaign{Name: "Spring"})

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content", nil))
	if !strings.Contains(rec.Body.String(), "badge-sponsor") {
		t.Error("board missing sponsored badge after placement")
	}
}
