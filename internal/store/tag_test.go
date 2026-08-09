package store

import (
	"context"
	"testing"
)

func TestTagsLibraryAndAssignment(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "V")

	if _, err := GetOrCreateTag(ctx, db, ch.ID, "  "); err != ErrInvalidTag {
		t.Errorf("blank tag err = %v, want ErrInvalidTag", err)
	}

	a, err := GetOrCreateTag(ctx, db, ch.ID, "Privacy")
	if err != nil {
		t.Fatalf("GetOrCreateTag: %v", err)
	}
	// Case-insensitive dedup returns the same row.
	again, err := GetOrCreateTag(ctx, db, ch.ID, "privacy")
	if err != nil {
		t.Fatalf("GetOrCreateTag (dup): %v", err)
	}
	if again.ID != a.ID {
		t.Errorf("case-insensitive dup created a new tag: %d vs %d", again.ID, a.ID)
	}

	b, _ := GetOrCreateTag(ctx, db, ch.ID, "VPN")
	lib, _ := ListTagsForChannel(ctx, db, ch.ID)
	if len(lib) != 2 {
		t.Fatalf("library size = %d, want 2", len(lib))
	}

	// Assign both (assignment is idempotent).
	if err := AssignTag(ctx, db, item.ID, a.ID); err != nil {
		t.Fatalf("AssignTag: %v", err)
	}
	_ = AssignTag(ctx, db, item.ID, a.ID) // idempotent
	if err := AssignTag(ctx, db, item.ID, b.ID); err != nil {
		t.Fatalf("AssignTag: %v", err)
	}
	assigned, _ := ListTagsForItem(ctx, db, item.ID)
	if len(assigned) != 2 {
		t.Fatalf("assigned = %d, want 2", len(assigned))
	}

	if err := UnassignTag(ctx, db, item.ID, a.ID); err != nil {
		t.Fatalf("UnassignTag: %v", err)
	}
	assigned, _ = ListTagsForItem(ctx, db, item.ID)
	if len(assigned) != 1 || assigned[0].Name != "VPN" {
		t.Errorf("after unassign: %+v", assigned)
	}
}
