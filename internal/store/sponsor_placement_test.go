package store

import (
	"context"
	"testing"
)

func TestPlacementAndDeliverables(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Sponsored Video")
	sp, _ := CreateSponsor(ctx, db, "Acme", "", "")
	camp, _ := CreateCampaign(ctx, db, Campaign{SponsorID: sp.ID, Name: "Spring"})

	// Attach.
	p, err := CreatePlacement(ctx, db, camp.ID, item.ID, "2026-03-15")
	if err != nil {
		t.Fatalf("CreatePlacement: %v", err)
	}
	// Duplicate attach is rejected.
	if _, err := CreatePlacement(ctx, db, camp.ID, item.ID, ""); err != ErrPlacementExists {
		t.Errorf("dup attach err = %v, want ErrPlacementExists", err)
	}

	list, err := ListPlacementsForItem(ctx, db, item.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListPlacementsForItem: %v (len=%d)", err, len(list))
	}
	if list[0].SponsorName != "Acme" || list[0].CampaignName != "Spring" || list[0].Deadline != "2026-03-15" {
		t.Errorf("unexpected placement: %+v", list[0])
	}

	// Deliverables.
	if _, err := AddDeliverable(ctx, db, p.ID, "  "); err != ErrInvalidDeliverable {
		t.Errorf("blank deliverable err = %v, want ErrInvalidDeliverable", err)
	}
	d, err := AddDeliverable(ctx, db, p.ID, "60s mid-roll read")
	if err != nil {
		t.Fatalf("AddDeliverable: %v", err)
	}
	if err := ToggleDeliverable(ctx, db, d.ID, p.ID); err != nil {
		t.Fatalf("ToggleDeliverable: %v", err)
	}
	ds, _ := ListDeliverables(ctx, db, p.ID)
	if len(ds) != 1 || !ds[0].Done {
		t.Fatalf("deliverable not toggled done: %+v", ds)
	}
	if err := ToggleDeliverable(ctx, db, d.ID, p.ID); err != nil {
		t.Fatalf("ToggleDeliverable back: %v", err)
	}
	ds, _ = ListDeliverables(ctx, db, p.ID)
	if ds[0].Done {
		t.Error("deliverable should be undone after second toggle")
	}

	if err := DeleteDeliverable(ctx, db, d.ID, p.ID); err != nil {
		t.Fatalf("DeleteDeliverable: %v", err)
	}
	if err := DeleteDeliverable(ctx, db, d.ID, p.ID); err != ErrDeliverableNotFound {
		t.Errorf("second delete err = %v, want ErrDeliverableNotFound", err)
	}

	// Deleting the sponsor cascades to placement + deliverables.
	if err := DeletePlacement(ctx, db, p.ID, item.ID); err != nil {
		t.Fatalf("DeletePlacement: %v", err)
	}
	list, _ = ListPlacementsForItem(ctx, db, item.ID)
	if len(list) != 0 {
		t.Errorf("placement not detached: %d remain", len(list))
	}
}
