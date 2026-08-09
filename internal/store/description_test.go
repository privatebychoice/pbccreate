package store

import (
	"context"
	"testing"
)

func TestGetDescriptionDefaultWhenMissing(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "D")

	d, err := GetDescription(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("GetDescription: %v", err)
	}
	if d.Intro != "" || d.ContentItemID != item.ID {
		t.Errorf("unexpected default: %+v", d)
	}
}

func TestSaveAndGetDescription(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "D")

	in := Description{
		ContentItemID: item.ID,
		Intro:         "Welcome back!",
		Chapters:      "0:00 Intro\n1:30 Body",
		Links:         "https://example.com",
		Hashtags:      "#privacy #video",
		Disclosure:    "Not sponsored.",
	}
	if _, err := SaveDescription(ctx, db, in); err != nil {
		t.Fatalf("SaveDescription: %v", err)
	}
	// Upsert: a second save updates in place.
	in.Intro = "Updated intro"
	saved, err := SaveDescription(ctx, db, in)
	if err != nil {
		t.Fatalf("SaveDescription update: %v", err)
	}
	if saved.Intro != "Updated intro" {
		t.Errorf("Intro = %q, want updated", saved.Intro)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM descriptions WHERE content_item_id = ?`, item.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("rows = %d, want 1 (upsert)", count)
	}
}

func TestDescriptionRender(t *testing.T) {
	d := Description{
		Intro:      "  Hello  ",
		Chapters:   "0:00 Intro",
		Links:      "",
		Disclosure: "Affiliate links below.",
		Hashtags:   "#tag",
	}
	want := "Hello\n\n0:00 Intro\n\nAffiliate links below.\n\n#tag"
	if got := d.Render(); got != want {
		t.Errorf("Render() =\n%q\nwant\n%q", got, want)
	}

	if got := (Description{}).Render(); got != "" {
		t.Errorf("empty Render() = %q, want empty", got)
	}
}
