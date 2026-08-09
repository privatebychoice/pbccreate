package store

import (
	"context"
	"testing"
)

func TestSponsorCRUD(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	if _, err := CreateSponsor(ctx, db, "  ", "", ""); err != ErrInvalidSponsor {
		t.Errorf("empty name err = %v, want ErrInvalidSponsor", err)
	}

	sp, err := CreateSponsor(ctx, db, "Acme VPN", "ads@acme.example", "great fit")
	if err != nil {
		t.Fatalf("CreateSponsor: %v", err)
	}
	if sp.Contact != "ads@acme.example" {
		t.Errorf("contact = %q", sp.Contact)
	}

	got, err := GetSponsor(ctx, db, sp.ID)
	if err != nil || got.Name != "Acme VPN" {
		t.Fatalf("GetSponsor: %v (%+v)", err, got)
	}

	if err := UpdateSponsor(ctx, db, sp.ID, "Acme Privacy", "new@acme.example", "renamed"); err != nil {
		t.Fatalf("UpdateSponsor: %v", err)
	}
	got, _ = GetSponsor(ctx, db, sp.ID)
	if got.Name != "Acme Privacy" || got.Contact != "new@acme.example" {
		t.Errorf("update not applied: %+v", got)
	}

	list, err := ListSponsors(ctx, db)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSponsors: %v (len=%d)", err, len(list))
	}

	if err := DeleteSponsor(ctx, db, sp.ID); err != nil {
		t.Fatalf("DeleteSponsor: %v", err)
	}
	if _, err := GetSponsor(ctx, db, sp.ID); err != ErrSponsorNotFound {
		t.Errorf("get after delete err = %v, want ErrSponsorNotFound", err)
	}
}
