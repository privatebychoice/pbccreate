package store

import (
	"context"
	"testing"
)

func TestTakeTracking(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")
	shot, _ := AddShot(ctx, db, item.ID, Shot{Description: "Wide"})
	other, _ := AddShot(ctx, db, item.ID, Shot{Description: "Close"})

	// Ownership check.
	if ok, _ := ShotExists(ctx, db, shot.ID, item.ID); !ok {
		t.Fatal("ShotExists should be true for own shot")
	}
	if ok, _ := ShotExists(ctx, db, shot.ID, item.ID+999); ok {
		t.Error("ShotExists should be false across items")
	}

	// Add takes; rating clamps to 0..5.
	if err := AddTake(ctx, db, shot.ID, Take{Label: "Take 1", Rating: 9, Notes: "soft"}); err != nil {
		t.Fatalf("AddTake: %v", err)
	}
	if err := AddTake(ctx, db, shot.ID, Take{Label: "Take 2", Rating: 5, Circled: true}); err != nil {
		t.Fatalf("AddTake 2: %v", err)
	}
	takes, _ := ListTakesForShot(ctx, db, shot.ID)
	if len(takes) != 2 {
		t.Fatalf("want 2 takes, got %d", len(takes))
	}
	if takes[0].Rating != 5 {
		t.Errorf("rating not clamped: %d, want 5", takes[0].Rating)
	}
	if !takes[1].Circled {
		t.Errorf("take 2 should be circled: %+v", takes[1])
	}

	// TakesByShot groups by shot; the other shot has none.
	byShot, _ := TakesByShot(ctx, db, item.ID)
	if len(byShot[shot.ID]) != 2 || len(byShot[other.ID]) != 0 {
		t.Fatalf("TakesByShot grouping wrong: %+v", byShot)
	}

	// Toggle circle off on take 2.
	if err := ToggleTakeCircled(ctx, db, takes[1].ID, shot.ID); err != nil {
		t.Fatalf("ToggleTakeCircled: %v", err)
	}
	takes, _ = ListTakesForShot(ctx, db, shot.ID)
	if takes[1].Circled {
		t.Errorf("take should be un-circled after toggle")
	}

	// A take not on this shot is not found.
	if err := ToggleTakeCircled(ctx, db, takes[0].ID, other.ID); err != ErrTakeNotFound {
		t.Errorf("cross-shot toggle err = %v, want ErrTakeNotFound", err)
	}

	// Delete a take.
	if err := DeleteTake(ctx, db, takes[0].ID, shot.ID); err != nil {
		t.Fatalf("DeleteTake: %v", err)
	}
	if takes, _ := ListTakesForShot(ctx, db, shot.ID); len(takes) != 1 {
		t.Fatalf("want 1 take after delete, got %d", len(takes))
	}

	// Deleting the shot cascades its takes.
	if err := DeleteShot(ctx, db, shot.ID, item.ID); err != nil {
		t.Fatalf("DeleteShot: %v", err)
	}
	if takes, _ := ListTakesForShot(ctx, db, shot.ID); len(takes) != 0 {
		t.Errorf("takes should cascade on shot delete: %d remain", len(takes))
	}
}
