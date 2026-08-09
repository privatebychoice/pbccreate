package store

import (
	"context"
	"testing"
)

func TestGetMediaAssetAndSetPreview(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "M")
	a, _ := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "/m/clip.mp4", Kind: "video", Present: true})

	got, err := GetMediaAsset(ctx, db, a.ID, item.ID)
	if err != nil {
		t.Fatalf("GetMediaAsset: %v", err)
	}
	if got.Path != "/m/clip.mp4" || got.PreviewPath != "" {
		t.Errorf("unexpected asset: %+v", got)
	}

	if err := SetMediaPreview(ctx, db, a.ID, item.ID, "/data/previews/media-1.jpg"); err != nil {
		t.Fatalf("SetMediaPreview: %v", err)
	}
	got, _ = GetMediaAsset(ctx, db, a.ID, item.ID)
	if got.PreviewPath != "/data/previews/media-1.jpg" {
		t.Errorf("PreviewPath = %q, want set", got.PreviewPath)
	}

	// Wrong content item / missing id -> not found.
	if _, err := GetMediaAsset(ctx, db, a.ID, 999999); err != ErrMediaNotFound {
		t.Errorf("cross-item get err = %v, want ErrMediaNotFound", err)
	}
	if err := SetMediaPreview(ctx, db, 999999, item.ID, "/x.jpg"); err != ErrMediaNotFound {
		t.Errorf("SetMediaPreview missing err = %v, want ErrMediaNotFound", err)
	}
}
