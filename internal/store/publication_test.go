package store

import (
	"context"
	"testing"
)

func TestPublicationCRUD(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")

	if _, err := CreatePublication(ctx, db, Publication{ContentItemID: item.ID, Platform: "  "}); err != ErrInvalidPublication {
		t.Errorf("empty platform err = %v, want ErrInvalidPublication", err)
	}

	// Unknown visibility normalizes to "".
	p, err := CreatePublication(ctx, db, Publication{
		ContentItemID:  item.ID,
		Platform:       "YouTube",
		PublishedTitle: "Best VPN 2026",
		ExternalID:     "abc123",
		OutputFile:     "/masters/final.mp4",
		Visibility:     "bogus",
	})
	if err != nil {
		t.Fatalf("CreatePublication: %v", err)
	}
	if p.Visibility != "" {
		t.Errorf("visibility = %q, want normalized to empty", p.Visibility)
	}

	got, err := GetPublication(ctx, db, p.ID, item.ID)
	if err != nil || got.OutputFile != "/masters/final.mp4" {
		t.Fatalf("GetPublication: %v (%+v)", err, got)
	}

	// Cross-item scoping.
	if _, err := GetPublication(ctx, db, p.ID, item.ID+999); err != ErrPublicationNotFound {
		t.Errorf("cross-item get err = %v, want ErrPublicationNotFound", err)
	}

	got.Visibility = "unlisted"
	got.URL = "https://youtu.be/abc123"
	if err := UpdatePublication(ctx, db, got); err != nil {
		t.Fatalf("UpdatePublication: %v", err)
	}
	after, _ := GetPublication(ctx, db, p.ID, item.ID)
	if after.Visibility != "unlisted" || after.URL != "https://youtu.be/abc123" {
		t.Fatalf("update not applied: %+v", after)
	}

	// A second platform is allowed for the same item.
	if _, err := CreatePublication(ctx, db, Publication{ContentItemID: item.ID, Platform: "Blog"}); err != nil {
		t.Fatalf("second publication: %v", err)
	}
	list, err := ListPublications(ctx, db, item.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListPublications: %v (n=%d)", err, len(list))
	}

	if err := DeletePublication(ctx, db, p.ID, item.ID); err != nil {
		t.Fatalf("DeletePublication: %v", err)
	}
	if _, err := GetPublication(ctx, db, p.ID, item.ID); err != ErrPublicationNotFound {
		t.Errorf("after delete err = %v, want ErrPublicationNotFound", err)
	}
}
