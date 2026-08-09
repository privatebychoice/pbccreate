package store

import (
	"context"
	"testing"
)

func TestLabelsAssignmentAndByItem(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	a, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "A")
	b, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "B")

	if _, err := GetOrCreateLabel(ctx, db, ch.ID, "  ", "blue"); err != ErrInvalidLabel {
		t.Errorf("blank label err = %v, want ErrInvalidLabel", err)
	}

	// Invalid color normalizes to blue.
	ever, err := GetOrCreateLabel(ctx, db, ch.ID, "Evergreen", "chartreuse")
	if err != nil {
		t.Fatalf("GetOrCreateLabel: %v", err)
	}
	if ever.Color != "blue" {
		t.Errorf("color = %q, want blue (normalized)", ever.Color)
	}
	// Case-insensitive dedup.
	dup, _ := GetOrCreateLabel(ctx, db, ch.ID, "evergreen", "red")
	if dup.ID != ever.ID {
		t.Errorf("dup created new label: %d vs %d", dup.ID, ever.ID)
	}

	reshoot, _ := GetOrCreateLabel(ctx, db, ch.ID, "Needs reshoot", "amber")
	_ = AssignLabel(ctx, db, a.ID, ever.ID)
	_ = AssignLabel(ctx, db, a.ID, reshoot.ID)
	_ = AssignLabel(ctx, db, b.ID, ever.ID)

	byItem, err := LabelsByItem(ctx, db)
	if err != nil {
		t.Fatalf("LabelsByItem: %v", err)
	}
	if len(byItem[a.ID]) != 2 || len(byItem[b.ID]) != 1 {
		t.Fatalf("byItem counts: a=%d b=%d", len(byItem[a.ID]), len(byItem[b.ID]))
	}

	if err := UnassignLabel(ctx, db, a.ID, reshoot.ID); err != nil {
		t.Fatalf("UnassignLabel: %v", err)
	}
	forA, _ := ListLabelsForItem(ctx, db, a.ID)
	if len(forA) != 1 || forA[0].Name != "Evergreen" {
		t.Errorf("after unassign: %+v", forA)
	}

	all, _ := ListAllLabels(ctx, db)
	if len(all) != 2 {
		t.Errorf("all labels = %d, want 2", len(all))
	}
}
