package store

import (
	"context"
	"testing"
)

func TestAssetProviderCRUD(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	if _, err := CreateAssetProvider(ctx, db, AssetProvider{Name: "   "}); err != ErrInvalidProvider {
		t.Errorf("empty name err = %v, want ErrInvalidProvider", err)
	}

	// Unknown service type / status normalize to defaults; fields are trimmed.
	p, err := CreateAssetProvider(ctx, db, AssetProvider{
		Name:        "  Epidemic Sound  ",
		ServiceType: "bogus",
		Status:      "",
		WebsiteURL:  " https://example.test ",
		RenewalOn:   "2026-12-01",
	})
	if err != nil {
		t.Fatalf("CreateAssetProvider: %v", err)
	}
	if p.Name != "Epidemic Sound" {
		t.Errorf("name = %q, want trimmed", p.Name)
	}
	if p.ServiceType != "other" {
		t.Errorf("service type = %q, want defaulted to other", p.ServiceType)
	}
	if p.Status != "active" {
		t.Errorf("status = %q, want defaulted to active", p.Status)
	}
	if p.WebsiteURL != "https://example.test" {
		t.Errorf("website = %q, want trimmed", p.WebsiteURL)
	}

	got, err := GetAssetProvider(ctx, db, p.ID)
	if err != nil || got.Name != "Epidemic Sound" || got.RenewalOn != "2026-12-01" {
		t.Fatalf("GetAssetProvider: %v (%+v)", err, got)
	}

	// Update to a valid service type + lapsed status.
	got.ServiceType = "music"
	got.Status = "lapsed"
	got.PlanTier = "Personal"
	if err := UpdateAssetProvider(ctx, db, got); err != nil {
		t.Fatalf("UpdateAssetProvider: %v", err)
	}
	after, _ := GetAssetProvider(ctx, db, p.ID)
	if after.ServiceType != "music" || after.Status != "lapsed" || after.PlanTier != "Personal" {
		t.Fatalf("update not applied: %+v", after)
	}

	list, err := ListAssetProviders(ctx, db)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAssetProviders: %v (n=%d)", err, len(list))
	}

	if err := DeleteAssetProvider(ctx, db, p.ID); err != nil {
		t.Fatalf("DeleteAssetProvider: %v", err)
	}
	if _, err := GetAssetProvider(ctx, db, p.ID); err != ErrProviderNotFound {
		t.Errorf("after delete err = %v, want ErrProviderNotFound", err)
	}
	if err := UpdateAssetProvider(ctx, db, AssetProvider{ID: p.ID, Name: "ghost"}); err != ErrProviderNotFound {
		t.Errorf("update missing err = %v, want ErrProviderNotFound", err)
	}
}
