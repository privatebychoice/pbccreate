package store

import (
	"context"
	"testing"
)

func TestShotAddListValidation(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "multi_cam", "Shots")

	if _, err := AddShot(ctx, db, item.ID, Shot{Description: "Wide establishing", Camera: "A", Framing: "WS"}); err != nil {
		t.Fatalf("AddShot: %v", err)
	}
	if _, err := AddShot(ctx, db, item.ID, Shot{Description: "Close up", Camera: "B", Status: "shot"}); err != nil {
		t.Fatalf("AddShot: %v", err)
	}

	shots, err := ListShots(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("ListShots: %v", err)
	}
	if len(shots) != 2 {
		t.Fatalf("len = %d, want 2", len(shots))
	}
	if shots[0].Description != "Wide establishing" || shots[0].Status != "planned" {
		t.Errorf("shot[0] = %+v", shots[0])
	}
	if shots[1].Status != "shot" || shots[1].Camera != "B" {
		t.Errorf("shot[1] = %+v", shots[1])
	}

	// Validation.
	if _, err := AddShot(ctx, db, item.ID, Shot{Description: "  "}); err != ErrInvalidShot {
		t.Errorf("empty desc err = %v, want ErrInvalidShot", err)
	}
	if _, err := AddShot(ctx, db, item.ID, Shot{Description: "x", Status: "bogus"}); err != ErrInvalidShotStatus {
		t.Errorf("bad status err = %v, want ErrInvalidShotStatus", err)
	}
}

func TestShotUpdateStatusAndDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Shots")
	shot, _ := AddShot(ctx, db, item.ID, Shot{Description: "A shot"})

	if err := UpdateShotStatus(ctx, db, shot.ID, item.ID, "selected"); err != nil {
		t.Fatalf("UpdateShotStatus: %v", err)
	}
	shots, _ := ListShots(ctx, db, item.ID)
	if shots[0].Status != "selected" {
		t.Errorf("status = %q, want selected", shots[0].Status)
	}
	if err := UpdateShotStatus(ctx, db, shot.ID, item.ID, "bogus"); err != ErrInvalidShotStatus {
		t.Errorf("err = %v, want ErrInvalidShotStatus", err)
	}
	if err := UpdateShotStatus(ctx, db, 999999, item.ID, "shot"); err != ErrShotNotFound {
		t.Errorf("err = %v, want ErrShotNotFound", err)
	}

	if err := DeleteShot(ctx, db, shot.ID, item.ID); err != nil {
		t.Fatalf("DeleteShot: %v", err)
	}
	if err := DeleteShot(ctx, db, shot.ID, item.ID); err != ErrShotNotFound {
		t.Errorf("second delete err = %v, want ErrShotNotFound", err)
	}
}

func TestShotMove(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Shots")
	_, _ = AddShot(ctx, db, item.ID, Shot{Description: "one"})
	two, _ := AddShot(ctx, db, item.ID, Shot{Description: "two"})

	if err := MoveShot(ctx, db, two.ID, item.ID, "up"); err != nil {
		t.Fatalf("MoveShot: %v", err)
	}
	shots, _ := ListShots(ctx, db, item.ID)
	if shots[0].Description != "two" || shots[1].Description != "one" {
		t.Errorf("order = %q,%q want two,one", shots[0].Description, shots[1].Description)
	}
}
