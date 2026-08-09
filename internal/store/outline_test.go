package store

import (
	"context"
	"testing"
)

func TestOutlineAddAndList(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "Outlined")

	if _, err := AddOutlineSegment(ctx, db, item.ID, "Hook", "grab attention", 15); err != nil {
		t.Fatalf("AddOutlineSegment: %v", err)
	}
	if _, err := AddOutlineSegment(ctx, db, item.ID, "Body", "", 0); err != nil {
		t.Fatalf("AddOutlineSegment: %v", err)
	}

	segs, err := ListOutlineSegments(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("ListOutlineSegments: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("len = %d, want 2", len(segs))
	}
	if segs[0].Title != "Hook" || segs[1].Title != "Body" {
		t.Errorf("order = %q, %q", segs[0].Title, segs[1].Title)
	}
	if segs[0].Position >= segs[1].Position {
		t.Errorf("positions not increasing: %d, %d", segs[0].Position, segs[1].Position)
	}
	if segs[0].TargetSeconds != 15 {
		t.Errorf("target = %d, want 15", segs[0].TargetSeconds)
	}
	if segs[1].TargetSeconds != 0 {
		t.Errorf("unset target = %d, want 0", segs[1].TargetSeconds)
	}
}

func TestOutlineAddValidation(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "V")

	if _, err := AddOutlineSegment(ctx, db, item.ID, "   ", "notes", 0); err != ErrInvalidSegment {
		t.Errorf("err = %v, want ErrInvalidSegment", err)
	}
}

func TestOutlineDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "V")
	seg, _ := AddOutlineSegment(ctx, db, item.ID, "Temp", "", 0)

	if err := DeleteOutlineSegment(ctx, db, seg.ID, item.ID); err != nil {
		t.Fatalf("DeleteOutlineSegment: %v", err)
	}
	if err := DeleteOutlineSegment(ctx, db, seg.ID, item.ID); err != ErrOutlineSegmentNotFound {
		t.Errorf("second delete err = %v, want ErrOutlineSegmentNotFound", err)
	}
}

func TestOutlineMove(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "V")
	_, _ = AddOutlineSegment(ctx, db, item.ID, "A", "", 0)
	b, _ := AddOutlineSegment(ctx, db, item.ID, "B", "", 0)
	_, _ = AddOutlineSegment(ctx, db, item.ID, "C", "", 0)

	// Move B up -> order B, A, C.
	if err := MoveOutlineSegment(ctx, db, b.ID, item.ID, "up"); err != nil {
		t.Fatalf("MoveOutlineSegment: %v", err)
	}
	segs, _ := ListOutlineSegments(ctx, db, item.ID)
	got := []string{segs[0].Title, segs[1].Title, segs[2].Title}
	want := []string{"B", "A", "C"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}

	// Moving the first item up is a no-op (no error, order unchanged).
	if err := MoveOutlineSegment(ctx, db, b.ID, item.ID, "up"); err != nil {
		t.Fatalf("edge move: %v", err)
	}
	segs, _ = ListOutlineSegments(ctx, db, item.ID)
	if segs[0].Title != "B" {
		t.Errorf("edge move changed order: first = %q", segs[0].Title)
	}

	// Invalid direction.
	if err := MoveOutlineSegment(ctx, db, b.ID, item.ID, "sideways"); err != ErrInvalidMove {
		t.Errorf("err = %v, want ErrInvalidMove", err)
	}
}
