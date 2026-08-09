package store

import (
	"context"
	"testing"
)

func TestCampaignCRUDAndFinancials(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	sp, _ := CreateSponsor(ctx, db, "Acme", "", "")

	if _, err := CreateCampaign(ctx, db, Campaign{SponsorID: sp.ID, Name: " "}); err != ErrInvalidCampaign {
		t.Errorf("empty name err = %v, want ErrInvalidCampaign", err)
	}

	// Create with financials + an invalid status (should normalize to "").
	c, err := CreateCampaign(ctx, db, Campaign{
		SponsorID:     sp.ID,
		Name:          "Spring Push",
		StartsOn:      "2026-03-01",
		EndsOn:        "2026-03-31",
		PromoCode:     "TUL10",
		Rate:          1500,
		RateSet:       true,
		Currency:      "USD",
		InvoiceStatus: "sent",
		PaymentStatus: "bogus", // invalid -> ""
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if c.PaymentStatus != "" {
		t.Errorf("invalid payment status not normalized: %q", c.PaymentStatus)
	}

	got, err := GetCampaign(ctx, db, c.ID, sp.ID)
	if err != nil {
		t.Fatalf("GetCampaign: %v", err)
	}
	if !got.RateSet || got.Rate != 1500 || got.InvoiceStatus != "sent" || got.PromoCode != "TUL10" {
		t.Errorf("unexpected campaign: %+v", got)
	}

	// Rate unset round-trips as RateSet=false.
	c2, _ := CreateCampaign(ctx, db, Campaign{SponsorID: sp.ID, Name: "No Money"})
	got2, _ := GetCampaign(ctx, db, c2.ID, sp.ID)
	if got2.RateSet {
		t.Errorf("expected RateSet=false, got rate %v", got2.Rate)
	}

	// Update.
	got.Name = "Spring Push v2"
	got.PaymentStatus = "paid"
	got.RateSet = false // clear the rate
	if err := UpdateCampaign(ctx, db, got); err != nil {
		t.Fatalf("UpdateCampaign: %v", err)
	}
	reread, _ := GetCampaign(ctx, db, c.ID, sp.ID)
	if reread.Name != "Spring Push v2" || reread.PaymentStatus != "paid" || reread.RateSet {
		t.Errorf("update not applied: %+v", reread)
	}

	list, _ := ListCampaigns(ctx, db, sp.ID)
	if len(list) != 2 {
		t.Fatalf("ListCampaigns = %d, want 2", len(list))
	}

	if err := DeleteCampaign(ctx, db, c.ID, sp.ID); err != nil {
		t.Fatalf("DeleteCampaign: %v", err)
	}
	if _, err := GetCampaign(ctx, db, c.ID, sp.ID); err != ErrCampaignNotFound {
		t.Errorf("get after delete err = %v, want ErrCampaignNotFound", err)
	}
}
