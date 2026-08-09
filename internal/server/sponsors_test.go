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

func TestSponsorCreateAndDetailFlow(t *testing.T) {
	s := newTestServerWithDB(t)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/sponsors", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Create a sponsor -> redirect to its detail page.
	rec := postForm(t, s, "/sponsors", token, url.Values{"name": {"Acme VPN"}, "contact": {"ads@acme.example"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create sponsor = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/sponsors/") {
		t.Fatalf("redirect = %q, want sponsor detail", loc)
	}

	// Detail page shows the sponsor and its (empty) campaigns section.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, loc, nil))
	if detRec.Code != http.StatusOK {
		t.Fatalf("detail = %d, want 200", detRec.Code)
	}
	if !strings.Contains(detRec.Body.String(), "Acme VPN") || !strings.Contains(detRec.Body.String(), "Campaigns") {
		t.Error("detail missing sponsor name or campaigns section")
	}
}

func TestSponsorCreateValidation(t *testing.T) {
	s := newTestServerWithDB(t)
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/sponsors", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, "/sponsors", token, url.Values{"name": {"   "}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Sponsor name is required") {
		t.Error("missing validation message")
	}
}

func TestCampaignCreateAndEditFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	sp, err := store.CreateSponsor(context.Background(), s.db, "Acme", "", "")
	if err != nil {
		t.Fatalf("CreateSponsor: %v", err)
	}
	sponsorURL := "/sponsors/" + strconv.FormatInt(sp.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, sponsorURL, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Create a campaign (name only) -> redirect to the campaign editor.
	createRec := postForm(t, s, sponsorURL+"/campaigns", token, url.Values{"name": {"Spring Push"}})
	if createRec.Code != http.StatusSeeOther {
		t.Fatalf("create campaign = %d, want 303", createRec.Code)
	}
	editURL := createRec.Header().Get("Location")
	if !strings.Contains(editURL, "/campaigns/") {
		t.Fatalf("redirect = %q, want campaign editor", editURL)
	}

	// Save full campaign details including financials.
	saveRec := postForm(t, s, editURL, token, url.Values{
		"name":           {"Spring Push"},
		"starts_on":      {"2026-03-01"},
		"ends_on":        {"2026-03-31"},
		"promo_code":     {"TUL10"},
		"rate":           {"1500"},
		"currency":       {"USD"},
		"invoice_status": {"sent"},
		"payment_status": {"unpaid"},
	})
	if saveRec.Code != http.StatusSeeOther {
		t.Fatalf("save campaign = %d, want 303", saveRec.Code)
	}

	campaigns, _ := store.ListCampaigns(context.Background(), s.db, sp.ID)
	if len(campaigns) != 1 {
		t.Fatalf("want 1 campaign, got %d", len(campaigns))
	}
	c := campaigns[0]
	if !c.RateSet || c.Rate != 1500 || c.PromoCode != "TUL10" || c.InvoiceStatus != "sent" || c.PaymentStatus != "unpaid" {
		t.Errorf("campaign not saved correctly: %+v", c)
	}
}
