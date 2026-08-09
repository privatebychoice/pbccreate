package store

import (
	"context"
	"testing"
)

func TestShootProfilesCRUDAndAssign(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")

	if _, err := CreateShootProfile(ctx, db, ch.ID, "gear", "  ", ""); err != ErrInvalidProfile {
		t.Errorf("empty name err = %v, want ErrInvalidProfile", err)
	}

	// Unknown kind normalizes to gear.
	g, err := CreateShootProfile(ctx, db, ch.ID, "bogus", "A-cam kit", "camera + 24-70")
	if err != nil {
		t.Fatalf("CreateShootProfile: %v", err)
	}
	if g.Kind != "gear" {
		t.Errorf("kind = %q, want normalized gear", g.Kind)
	}
	loc, _ := CreateShootProfile(ctx, db, ch.ID, "location", "Home studio", "quiet, good light")

	// get-or-create is case-insensitive within channel+kind (no duplicate).
	again, _ := GetOrCreateShootProfile(ctx, db, ch.ID, "gear", "a-cam kit")
	if again.ID != g.ID {
		t.Errorf("get-or-create duplicated gear: %d vs %d", again.ID, g.ID)
	}
	// Same name under a different kind is a distinct profile.
	locByName, _ := GetOrCreateShootProfile(ctx, db, ch.ID, "location", "A-cam kit")
	if locByName.ID == g.ID {
		t.Error("gear and location namespaces should be separate")
	}

	// Kind-filtered listing.
	gears, _ := ListShootProfiles(ctx, db, "gear")
	if len(gears) != 1 || gears[0].Name != "A-cam kit" {
		t.Fatalf("gear list wrong: %+v", gears)
	}
	locs, _ := ListShootProfiles(ctx, db, "location")
	if len(locs) != 2 { // "Home studio" + "A-cam kit" (location)
		t.Fatalf("location list wrong: %+v", locs)
	}

	// Assign gear + location to the item; ListProfilesForItem filters by kind.
	if err := AssignProfile(ctx, db, item.ID, g.ID); err != nil {
		t.Fatalf("AssignProfile gear: %v", err)
	}
	_ = AssignProfile(ctx, db, item.ID, g.ID) // idempotent
	_ = AssignProfile(ctx, db, item.ID, loc.ID)

	itemGear, _ := ListProfilesForItem(ctx, db, item.ID, "gear")
	if len(itemGear) != 1 || itemGear[0].Name != "A-cam kit" {
		t.Fatalf("item gear wrong: %+v", itemGear)
	}
	itemLoc, _ := ListProfilesForItem(ctx, db, item.ID, "location")
	if len(itemLoc) != 1 || itemLoc[0].Name != "Home studio" {
		t.Fatalf("item location wrong: %+v", itemLoc)
	}

	// Unassign gear; it remains defined.
	if err := UnassignProfile(ctx, db, item.ID, g.ID); err != nil {
		t.Fatalf("UnassignProfile: %v", err)
	}
	if itemGear, _ := ListProfilesForItem(ctx, db, item.ID, "gear"); len(itemGear) != 0 {
		t.Errorf("gear not unassigned: %+v", itemGear)
	}
	if _, err := GetShootProfile(ctx, db, g.ID); err != nil {
		t.Errorf("gear profile should remain after unassign: %v", err)
	}

	// Update + delete (delete cascades assignment).
	if err := UpdateShootProfile(ctx, db, loc.ID, "Home studio v2", "treated"); err != nil {
		t.Fatalf("UpdateShootProfile: %v", err)
	}
	if got, _ := GetShootProfile(ctx, db, loc.ID); got.Name != "Home studio v2" {
		t.Errorf("update not applied: %+v", got)
	}
	if err := DeleteShootProfile(ctx, db, loc.ID); err != nil {
		t.Fatalf("DeleteShootProfile: %v", err)
	}
	if itemLoc, _ := ListProfilesForItem(ctx, db, item.ID, "location"); len(itemLoc) != 0 {
		t.Errorf("assignment should cascade on profile delete: %+v", itemLoc)
	}
}
