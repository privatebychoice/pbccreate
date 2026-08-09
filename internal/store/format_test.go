package store

import (
	"context"
	"testing"
)

func TestFormatCRUDSegmentsAndSeed(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")

	if _, err := CreateFormat(ctx, db, Format{ChannelID: ch.ID, Name: "  "}); err != ErrInvalidFormat {
		t.Errorf("empty name err = %v, want ErrInvalidFormat", err)
	}

	// Mode is dropped for a type that has no mode; kept for video.
	blog, _ := CreateFormat(ctx, db, Format{ChannelID: ch.ID, Name: "Essay", DefaultType: "blog", DefaultMode: "obs"})
	if blog.DefaultMode != "" {
		t.Errorf("blog mode = %q, want cleared", blog.DefaultMode)
	}
	f, err := CreateFormat(ctx, db, Format{ChannelID: ch.ID, Name: "Tutorial", DefaultType: "video", DefaultMode: "single_cam"})
	if err != nil {
		t.Fatalf("CreateFormat: %v", err)
	}
	if f.DefaultMode != "single_cam" {
		t.Errorf("video mode = %q, want single_cam", f.DefaultMode)
	}

	// Bogus type normalizes to video.
	if got := normFormatType("bogus"); got != "video" {
		t.Errorf("normFormatType(bogus) = %q, want video", got)
	}

	// Default outline: add three, reorder, delete one.
	if err := AddFormatSegment(ctx, db, f.ID, "  ", "", 0); err != ErrInvalidFormatSegment {
		t.Errorf("empty segment err = %v, want ErrInvalidFormatSegment", err)
	}
	_ = AddFormatSegment(ctx, db, f.ID, "Hook", "grab attention", 15)
	_ = AddFormatSegment(ctx, db, f.ID, "Body", "", 120)
	_ = AddFormatSegment(ctx, db, f.ID, "Outro", "CTA", 20)

	segs, _ := ListFormatSegments(ctx, db, f.ID)
	if len(segs) != 3 || segs[0].Title != "Hook" || segs[2].Title != "Outro" {
		t.Fatalf("segments wrong: %+v", segs)
	}
	if err := MoveFormatSegment(ctx, db, segs[2].ID, f.ID, "up"); err != nil {
		t.Fatalf("MoveFormatSegment: %v", err)
	}
	segs, _ = ListFormatSegments(ctx, db, f.ID)
	if segs[1].Title != "Outro" {
		t.Fatalf("order after move wrong: %+v", segs)
	}

	// List reports the segment count.
	list, _ := ListFormats(ctx, db)
	var tut FormatSummary
	for _, fs := range list {
		if fs.Name == "Tutorial" {
			tut = fs
		}
	}
	if tut.SegmentCount != 3 {
		t.Errorf("segment count = %d, want 3", tut.SegmentCount)
	}

	// Seed a content item: it copies type/mode and the outline in order.
	item, err := SeedContentItemFromFormat(ctx, db, f.ID, "My Tutorial")
	if err != nil {
		t.Fatalf("SeedContentItemFromFormat: %v", err)
	}
	if item.Type != "video" || item.Mode != "single_cam" || item.Status != "idea" || item.Title != "My Tutorial" {
		t.Fatalf("seeded item wrong: %+v", item)
	}
	outline, _ := ListOutlineSegments(ctx, db, item.ID)
	if len(outline) != 3 || outline[0].Title != "Hook" || outline[1].Title != "Outro" {
		t.Fatalf("seeded outline wrong: %+v", outline)
	}

	// Delete a segment, then delete the format (cascades remaining).
	if err := DeleteFormatSegment(ctx, db, segs[0].ID, f.ID); err != nil {
		t.Fatalf("DeleteFormatSegment: %v", err)
	}
	if err := DeleteFormat(ctx, db, f.ID); err != nil {
		t.Fatalf("DeleteFormat: %v", err)
	}
	if _, err := GetFormat(ctx, db, f.ID); err != ErrFormatNotFound {
		t.Errorf("after delete err = %v, want ErrFormatNotFound", err)
	}
	// The already-seeded content item and its outline survive the format delete.
	if outline, _ := ListOutlineSegments(ctx, db, item.ID); len(outline) != 3 {
		t.Errorf("seeded outline should survive format delete: %d", len(outline))
	}
}
