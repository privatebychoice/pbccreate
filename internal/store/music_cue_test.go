package store

import (
	"context"
	"testing"
)

func TestMusicCueCRUD(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")
	prov, _ := CreateAssetProvider(ctx, db, AssetProvider{Name: "Artlist", ServiceType: "music"})

	if _, err := AddMusicCue(ctx, db, MusicCue{ContentItemID: item.ID, Title: "  "}); err != ErrInvalidMusicCue {
		t.Errorf("empty title err = %v, want ErrInvalidMusicCue", err)
	}

	cue, err := AddMusicCue(ctx, db, MusicCue{
		ContentItemID: item.ID,
		ProviderID:    prov.ID,
		Title:         "Sunrise",
		Artist:        "Artist",
		InPoint:       "0:00",
		OutPoint:      "1:23",
		License:       "royalty-free",
	})
	if err != nil {
		t.Fatalf("AddMusicCue: %v", err)
	}
	_ = cue

	cues, _ := ListMusicCues(ctx, db, item.ID)
	if len(cues) != 1 {
		t.Fatalf("want 1 cue, got %d", len(cues))
	}
	got := cues[0]
	if got.Title != "Sunrise" || got.OutPoint != "1:23" || got.ProviderName != "Artlist" {
		t.Fatalf("cue fields wrong: %+v", got)
	}

	// Scoping.
	if _, err := GetMusicCue(ctx, db, got.ID, item.ID+999); err != ErrMusicCueNotFound {
		t.Errorf("cross-item get err = %v, want ErrMusicCueNotFound", err)
	}

	// Deleting the provider clears the cue link (SET NULL), not the cue.
	if err := DeleteAssetProvider(ctx, db, prov.ID); err != nil {
		t.Fatalf("DeleteAssetProvider: %v", err)
	}
	after, _ := GetMusicCue(ctx, db, got.ID, item.ID)
	if after.ProviderID != 0 || after.ProviderName != "" {
		t.Errorf("provider link not cleared: %+v", after)
	}

	if err := DeleteMusicCue(ctx, db, got.ID, item.ID); err != nil {
		t.Fatalf("DeleteMusicCue: %v", err)
	}
	if cues, _ := ListMusicCues(ctx, db, item.ID); len(cues) != 0 {
		t.Errorf("cue not deleted: %d remain", len(cues))
	}
}
