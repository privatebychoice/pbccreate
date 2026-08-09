package store

import (
	"context"
	"testing"
)

func TestPillarCRUDAssignAndCoverage(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")

	if _, err := CreatePillar(ctx, db, ch.ID, "  ", ""); err != ErrInvalidPillar {
		t.Errorf("empty name err = %v, want ErrInvalidPillar", err)
	}

	privacy, err := CreatePillar(ctx, db, ch.ID, "Privacy", "core theme")
	if err != nil {
		t.Fatalf("CreatePillar: %v", err)
	}
	if privacy.ChannelName != "TUL" {
		t.Errorf("channel name = %q, want TUL", privacy.ChannelName)
	}

	// GetOrCreate is case-insensitive: same pillar, not a duplicate.
	again, err := GetOrCreatePillar(ctx, db, ch.ID, "privacy")
	if err != nil {
		t.Fatalf("GetOrCreatePillar: %v", err)
	}
	if again.ID != privacy.ID {
		t.Errorf("get-or-create made a duplicate: %d vs %d", again.ID, privacy.ID)
	}
	// A new name creates a new pillar.
	tools, _ := GetOrCreatePillar(ctx, db, ch.ID, "Tools")

	// Assign two items to Privacy, none to Tools.
	a, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "A")
	b, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "B")
	if err := AssignPillar(ctx, db, a.ID, privacy.ID); err != nil {
		t.Fatalf("AssignPillar: %v", err)
	}
	if err := AssignPillar(ctx, db, a.ID, privacy.ID); err != nil { // idempotent
		t.Fatalf("AssignPillar (repeat): %v", err)
	}
	_ = AssignPillar(ctx, db, b.ID, privacy.ID)

	forItem, _ := ListPillarsForItem(ctx, db, a.ID)
	if len(forItem) != 1 || forItem[0].Name != "Privacy" {
		t.Fatalf("ListPillarsForItem: %+v", forItem)
	}

	cov, err := ListPillarCoverage(ctx, db)
	if err != nil || len(cov) != 2 {
		t.Fatalf("ListPillarCoverage: %v (n=%d)", err, len(cov))
	}
	counts := map[string]int{}
	for _, c := range cov {
		counts[c.Name] = c.ItemCount
	}
	if counts["Privacy"] != 2 || counts["Tools"] != 0 {
		t.Fatalf("coverage counts wrong: %+v", counts)
	}

	// Unassign one, coverage drops.
	if err := UnassignPillar(ctx, db, b.ID, privacy.ID); err != nil {
		t.Fatalf("UnassignPillar: %v", err)
	}
	cov, _ = ListPillarCoverage(ctx, db)
	for _, c := range cov {
		if c.Name == "Privacy" && c.ItemCount != 1 {
			t.Errorf("Privacy count = %d, want 1 after unassign", c.ItemCount)
		}
	}

	// Update + delete (delete cascades assignments).
	if err := UpdatePillar(ctx, db, tools.ID, "Gear", "hardware reviews"); err != nil {
		t.Fatalf("UpdatePillar: %v", err)
	}
	renamed, _ := GetPillar(ctx, db, tools.ID)
	if renamed.Name != "Gear" {
		t.Errorf("rename not applied: %+v", renamed)
	}
	if err := DeletePillar(ctx, db, privacy.ID); err != nil {
		t.Fatalf("DeletePillar: %v", err)
	}
	if _, err := GetPillar(ctx, db, privacy.ID); err != ErrPillarNotFound {
		t.Errorf("after delete err = %v, want ErrPillarNotFound", err)
	}
	// Its assignment is gone too.
	forItem, _ = ListPillarsForItem(ctx, db, a.ID)
	if len(forItem) != 0 {
		t.Errorf("assignment not cascaded on pillar delete: %+v", forItem)
	}
}
