package store

import (
	"context"
	"testing"
)

func TestGetContentItem(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	created, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "Threat Modeling 101")

	got, err := GetContentItem(ctx, db, created.ID)
	if err != nil {
		t.Fatalf("GetContentItem: %v", err)
	}
	if got.Title != "Threat Modeling 101" || got.ChannelName != "TUL" || got.Mode != "faceless" {
		t.Errorf("unexpected item: %+v", got)
	}

	if _, err := GetContentItem(ctx, db, 999999); err != ErrContentItemNotFound {
		t.Errorf("missing id err = %v, want ErrContentItemNotFound", err)
	}
}

func TestUpdateContentItemStatus(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "A Video")

	if err := UpdateContentItemStatus(ctx, db, item.ID, "editing"); err != nil {
		t.Fatalf("UpdateContentItemStatus: %v", err)
	}
	got, _ := GetContentItem(ctx, db, item.ID)
	if got.Status != "editing" {
		t.Errorf("status = %q, want editing", got.Status)
	}

	if err := UpdateContentItemStatus(ctx, db, item.ID, "bogus"); err != ErrInvalidStatus {
		t.Errorf("invalid status err = %v, want ErrInvalidStatus", err)
	}
	if err := UpdateContentItemStatus(ctx, db, 999999, "idea"); err != ErrContentItemNotFound {
		t.Errorf("missing id err = %v, want ErrContentItemNotFound", err)
	}
}
