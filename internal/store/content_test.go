package store

import (
	"context"
	"testing"
)

func TestCreateAndListContentItems(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	ch, err := CreateChannel(ctx, db, "TUL", "youtube")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	item, err := CreateContentItem(ctx, db, ch.ID, "video", "single_cam", "My First Video")
	if err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}
	if item.Status != "idea" {
		t.Errorf("status = %q, want idea", item.Status)
	}

	got, err := ListContentItems(ctx, db)
	if err != nil {
		t.Fatalf("ListContentItems: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ChannelName != "TUL" {
		t.Errorf("ChannelName = %q, want TUL", got[0].ChannelName)
	}
	if got[0].Mode != "single_cam" {
		t.Errorf("Mode = %q, want single_cam", got[0].Mode)
	}
}

func TestCreateContentItemModeClearedForNonVideo(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "PBC", "blog")

	item, err := CreateContentItem(ctx, db, ch.ID, "blog", "faceless", "A Blog Post")
	if err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}
	if item.Mode != "" {
		t.Errorf("Mode = %q, want empty for blog type", item.Mode)
	}
}

func TestCreateContentItemValidation(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")

	cases := []struct {
		name             string
		channelID        int64
		typ, mode, title string
	}{
		{"empty title", ch.ID, "video", "", ""},
		{"invalid type", ch.ID, "podcast", "", "Title"},
		{"invalid mode", ch.ID, "video", "bogus", "Title"},
		{"no channel", 0, "video", "", "Title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CreateContentItem(ctx, db, tc.channelID, tc.typ, tc.mode, tc.title); err != ErrInvalidContentItem {
				t.Errorf("err = %v, want ErrInvalidContentItem", err)
			}
		})
	}
}
