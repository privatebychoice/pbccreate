package store

import (
	"context"
	"testing"
)

func TestAssetLibraryCRUDAndSearch(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	prov, _ := CreateAssetProvider(ctx, db, AssetProvider{Name: "Artlist", ServiceType: "music"})

	if _, err := CreateLibraryAsset(ctx, db, LibraryAsset{Name: "  "}); err != ErrInvalidLibraryAsset {
		t.Errorf("empty name err = %v, want ErrInvalidLibraryAsset", err)
	}

	// Unknown kind normalizes to other; provider link joins.
	music, err := CreateLibraryAsset(ctx, db, LibraryAsset{
		Kind: "music", Name: "Sunrise Theme", Tags: "calm, intro", License: "royalty-free", ProviderID: prov.ID,
	})
	if err != nil {
		t.Fatalf("CreateLibraryAsset: %v", err)
	}
	if music.ProviderName != "Artlist" {
		t.Errorf("provider join = %q, want Artlist", music.ProviderName)
	}
	_, _ = CreateLibraryAsset(ctx, db, LibraryAsset{Kind: "bogus", Name: "City drone shot", Tags: "urban, aerial"})
	_, _ = CreateLibraryAsset(ctx, db, LibraryAsset{Kind: "graphic", Name: "Logo lower-third"})

	// No filter → all three.
	all, _ := ListLibraryAssets(ctx, db, "", "")
	if len(all) != 3 {
		t.Fatalf("want 3 assets, got %d", len(all))
	}

	// Kind filter.
	if got, _ := ListLibraryAssets(ctx, db, "", "music"); len(got) != 1 || got[0].Name != "Sunrise Theme" {
		t.Fatalf("kind filter wrong: %+v", got)
	}
	// Bogus normalized to other → the drone shot.
	if got, _ := ListLibraryAssets(ctx, db, "", "other"); len(got) != 1 || got[0].Name != "City drone shot" {
		t.Fatalf("other-kind filter wrong: %+v", got)
	}

	// Query matches name or tags (case-insensitive).
	if got, _ := ListLibraryAssets(ctx, db, "aerial", ""); len(got) != 1 || got[0].Name != "City drone shot" {
		t.Fatalf("tag query wrong: %+v", got)
	}
	if got, _ := ListLibraryAssets(ctx, db, "sunrise", ""); len(got) != 1 {
		t.Fatalf("name query wrong: %+v", got)
	}
	// Combined kind + query.
	if got, _ := ListLibraryAssets(ctx, db, "calm", "music"); len(got) != 1 {
		t.Fatalf("combined filter wrong: %+v", got)
	}
	if got, _ := ListLibraryAssets(ctx, db, "nomatch", ""); len(got) != 0 {
		t.Fatalf("no-match query should be empty: %+v", got)
	}

	// Update + provider-delete clears link.
	music.Tags = "calm, intro, cinematic"
	if err := UpdateLibraryAsset(ctx, db, music); err != nil {
		t.Fatalf("UpdateLibraryAsset: %v", err)
	}
	if err := DeleteAssetProvider(ctx, db, prov.ID); err != nil {
		t.Fatalf("DeleteAssetProvider: %v", err)
	}
	after, _ := GetLibraryAsset(ctx, db, music.ID)
	if after.ProviderID != 0 || after.ProviderName != "" {
		t.Errorf("provider link not cleared: %+v", after)
	}

	// Delete.
	if err := DeleteLibraryAsset(ctx, db, music.ID); err != nil {
		t.Fatalf("DeleteLibraryAsset: %v", err)
	}
	if _, err := GetLibraryAsset(ctx, db, music.ID); err != ErrLibraryAssetNotFound {
		t.Errorf("after delete err = %v, want ErrLibraryAssetNotFound", err)
	}
}
