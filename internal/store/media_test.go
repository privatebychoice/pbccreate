package store

import (
	"context"
	"testing"
	"time"
)

func TestMediaAddListDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "single_cam", "Media")

	a, err := AddMediaAsset(ctx, db, MediaAsset{
		ContentItemID: item.ID,
		Path:          "/media/footage/aroll.mov",
		Kind:          "video",
		Status:        "recorded",
		Present:       true,
		SizeBytes:     123456,
		MTime:         time.Now().UTC().Truncate(time.Second),
	})
	if err != nil {
		t.Fatalf("AddMediaAsset: %v", err)
	}
	if a.Filename != "aroll.mov" {
		t.Errorf("Filename = %q, want derived aroll.mov", a.Filename)
	}

	// Defaults applied when kind/status omitted.
	if _, err := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "/media/footage/notes.txt"}); err != nil {
		t.Fatalf("AddMediaAsset defaults: %v", err)
	}

	list, err := ListMediaAssets(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("ListMediaAssets: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}

	if err := DeleteMediaAsset(ctx, db, a.ID, item.ID); err != nil {
		t.Fatalf("DeleteMediaAsset: %v", err)
	}
	if err := DeleteMediaAsset(ctx, db, a.ID, item.ID); err != ErrMediaNotFound {
		t.Errorf("second delete err = %v, want ErrMediaNotFound", err)
	}
}

func TestMediaValidation(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "M")

	if _, err := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "  "}); err != ErrInvalidMedia {
		t.Errorf("empty path err = %v, want ErrInvalidMedia", err)
	}
	if _, err := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "/x.mp4", Kind: "hologram"}); err != ErrInvalidMediaKind {
		t.Errorf("bad kind err = %v, want ErrInvalidMediaKind", err)
	}
	if _, err := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "/x.mp4", Status: "lost"}); err != ErrInvalidMediaStatus {
		t.Errorf("bad status err = %v, want ErrInvalidMediaStatus", err)
	}
}

func TestMediaStatusAndPresence(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "M")
	a, _ := AddMediaAsset(ctx, db, MediaAsset{ContentItemID: item.ID, Path: "/m/clip.mp4", Present: true})

	if err := UpdateMediaStatus(ctx, db, a.ID, item.ID, "edited"); err != nil {
		t.Fatalf("UpdateMediaStatus: %v", err)
	}
	if err := UpdateMediaStatus(ctx, db, a.ID, item.ID, "nope"); err != ErrInvalidMediaStatus {
		t.Errorf("bad status err = %v, want ErrInvalidMediaStatus", err)
	}

	// Mark missing.
	if err := SetMediaPresence(ctx, db, a.ID, item.ID, false, 0, time.Time{}); err != nil {
		t.Fatalf("SetMediaPresence: %v", err)
	}
	list, _ := ListMediaAssets(ctx, db, item.ID)
	if list[0].Present {
		t.Error("asset should be marked not present")
	}
	if list[0].Status != "edited" {
		t.Errorf("status = %q, want edited (unchanged by presence)", list[0].Status)
	}

	// Mark present again with fresh size/mtime.
	when := time.Now().UTC().Truncate(time.Second)
	if err := SetMediaPresence(ctx, db, a.ID, item.ID, true, 999, when); err != nil {
		t.Fatalf("SetMediaPresence present: %v", err)
	}
	list, _ = ListMediaAssets(ctx, db, item.ID)
	if !list[0].Present || list[0].SizeBytes != 999 {
		t.Errorf("after refresh: present=%v size=%d", list[0].Present, list[0].SizeBytes)
	}
}
