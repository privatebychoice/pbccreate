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

func TestShotBeatLinkAndUpdate(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "Beats")
	beat1, _ := AddOutlineSegment(ctx, db, item.ID, "The Hook", "", 15)
	beat2, _ := AddOutlineSegment(ctx, db, item.ID, "The Close", "", 10)

	// Add a shot linked to beat 1; the link round-trips through ListShots.
	sh, err := AddShot(ctx, db, item.ID, Shot{Description: "phone screen", OutlineSegmentID: beat1.ID})
	if err != nil {
		t.Fatalf("AddShot linked: %v", err)
	}
	shots, _ := ListShots(ctx, db, item.ID)
	if len(shots) != 1 || shots[0].OutlineSegmentID != beat1.ID {
		t.Fatalf("linked shot = %+v", shots)
	}

	// Adding with a beat from another item / bogus id is rejected.
	other, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "Other")
	otherBeat, _ := AddOutlineSegment(ctx, db, other.ID, "Foreign", "", 0)
	if _, err := AddShot(ctx, db, item.ID, Shot{Description: "x", OutlineSegmentID: otherBeat.ID}); err != ErrOutlineSegmentNotFound {
		t.Errorf("cross-item beat err = %v, want ErrOutlineSegmentNotFound", err)
	}

	// UpdateShot edits fields and relinks to beat 2.
	if err := UpdateShot(ctx, db, sh.ID, item.ID, Shot{Description: "phone lock screen", Scene: "Phone", OutlineSegmentID: beat2.ID}); err != nil {
		t.Fatalf("UpdateShot: %v", err)
	}
	shots, _ = ListShots(ctx, db, item.ID)
	if shots[0].Description != "phone lock screen" || shots[0].Scene != "Phone" || shots[0].OutlineSegmentID != beat2.ID {
		t.Fatalf("after update = %+v", shots[0])
	}

	// Unlink (0) clears the beat.
	if err := UpdateShot(ctx, db, sh.ID, item.ID, Shot{Description: "phone lock screen", OutlineSegmentID: 0}); err != nil {
		t.Fatalf("UpdateShot unlink: %v", err)
	}
	shots, _ = ListShots(ctx, db, item.ID)
	if shots[0].OutlineSegmentID != 0 {
		t.Errorf("expected unlinked, got %d", shots[0].OutlineSegmentID)
	}

	// UpdateShot validation: empty description, bogus beat, cross-item shot.
	if err := UpdateShot(ctx, db, sh.ID, item.ID, Shot{Description: "  "}); err != ErrInvalidShot {
		t.Errorf("empty desc err = %v, want ErrInvalidShot", err)
	}
	if err := UpdateShot(ctx, db, sh.ID, item.ID, Shot{Description: "x", OutlineSegmentID: otherBeat.ID}); err != ErrOutlineSegmentNotFound {
		t.Errorf("bogus beat err = %v, want ErrOutlineSegmentNotFound", err)
	}
	if err := UpdateShot(ctx, db, sh.ID, other.ID, Shot{Description: "x"}); err != ErrShotNotFound {
		t.Errorf("cross-item shot err = %v, want ErrShotNotFound", err)
	}
}

func TestShotBeatSetNullOnSegmentDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "Cascade")
	beat, _ := AddOutlineSegment(ctx, db, item.ID, "The Hook", "", 0)
	sh, _ := AddShot(ctx, db, item.ID, Shot{Description: "clip", OutlineSegmentID: beat.ID})

	// Deleting the beat must unlink the shot, not delete it (ON DELETE SET NULL).
	if err := DeleteOutlineSegment(ctx, db, beat.ID, item.ID); err != nil {
		t.Fatalf("DeleteOutlineSegment: %v", err)
	}
	shots, _ := ListShots(ctx, db, item.ID)
	if len(shots) != 1 {
		t.Fatalf("shot should survive segment delete, got %d", len(shots))
	}
	if shots[0].OutlineSegmentID != 0 {
		t.Errorf("shot beat = %d, want 0 after segment delete (id %d)", shots[0].OutlineSegmentID, sh.ID)
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
