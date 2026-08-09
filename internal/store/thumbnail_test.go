package store

import (
	"context"
	"testing"
)

func TestThumbnailCRUD(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Thumb")

	if _, err := CreateThumbnail(ctx, db, item.ID, "  ", "{}"); err != ErrInvalidThumbnail {
		t.Errorf("empty name err = %v, want ErrInvalidThumbnail", err)
	}

	th, err := CreateThumbnail(ctx, db, item.ID, "Main", `{"background":"#000000"}`)
	if err != nil {
		t.Fatalf("CreateThumbnail: %v", err)
	}

	got, err := GetThumbnail(ctx, db, th.ID, item.ID)
	if err != nil {
		t.Fatalf("GetThumbnail: %v", err)
	}
	if got.Name != "Main" || got.CanvasJSON != `{"background":"#000000"}` {
		t.Errorf("unexpected thumbnail: %+v", got)
	}

	if err := UpdateThumbnailCanvas(ctx, db, th.ID, item.ID, `{"background":"#ffffff"}`); err != nil {
		t.Fatalf("UpdateThumbnailCanvas: %v", err)
	}
	got, _ = GetThumbnail(ctx, db, th.ID, item.ID)
	if got.CanvasJSON != `{"background":"#ffffff"}` {
		t.Errorf("canvas not updated: %q", got.CanvasJSON)
	}

	list, err := ListThumbnails(ctx, db, item.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListThumbnails: %v (len=%d)", err, len(list))
	}

	if err := DeleteThumbnail(ctx, db, th.ID, item.ID); err != nil {
		t.Fatalf("DeleteThumbnail: %v", err)
	}
	if _, err := GetThumbnail(ctx, db, th.ID, item.ID); err != ErrThumbnailNotFound {
		t.Errorf("get after delete err = %v, want ErrThumbnailNotFound", err)
	}
}
