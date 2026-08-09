package store

import (
	"context"
	"testing"
)

func TestUpdateMediaMetadata(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "M")
	a, _ := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "/m/clip.mp4", Kind: "video", Present: true})

	// Metadata starts empty.
	got, _ := ListMediaAssets(ctx, db, item.ID)
	if got[0].Width != 0 || got[0].Codec != "" || got[0].DurationSeconds != 0 {
		t.Fatalf("expected empty metadata, got %+v", got[0])
	}

	if err := UpdateMediaMetadata(ctx, db, a.ID, item.ID, MediaMetadata{
		DurationSeconds: 90, Width: 1920, Height: 1080, Codec: "h264", FPS: 29.97, Container: "mov",
	}); err != nil {
		t.Fatalf("UpdateMediaMetadata: %v", err)
	}

	got, _ = ListMediaAssets(ctx, db, item.ID)
	m := got[0]
	if m.DurationSeconds != 90 || m.Width != 1920 || m.Height != 1080 || m.Codec != "h264" || m.Container != "mov" {
		t.Errorf("metadata not stored: %+v", m)
	}
	if m.FPS < 29.9 || m.FPS > 30.0 {
		t.Errorf("FPS = %v, want ~29.97", m.FPS)
	}

	if err := UpdateMediaMetadata(ctx, db, 999999, item.ID, MediaMetadata{}); err != ErrMediaNotFound {
		t.Errorf("err = %v, want ErrMediaNotFound", err)
	}
}
