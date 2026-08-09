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

	// Default shot-list: add two, reorder, and confirm.
	if err := AddFormatShot(ctx, db, f.ID, FormatShot{Description: "  "}); err != ErrInvalidFormatShot {
		t.Errorf("empty shot err = %v, want ErrInvalidFormatShot", err)
	}
	_ = AddFormatShot(ctx, db, f.ID, FormatShot{Description: "Wide establishing", Scene: "int", Camera: "A"})
	_ = AddFormatShot(ctx, db, f.ID, FormatShot{Description: "Close up"})
	fshots, _ := ListFormatShots(ctx, db, f.ID)
	if len(fshots) != 2 || fshots[0].Description != "Wide establishing" {
		t.Fatalf("format shots wrong: %+v", fshots)
	}
	if err := MoveFormatShot(ctx, db, fshots[1].ID, f.ID, "up"); err != nil {
		t.Fatalf("MoveFormatShot: %v", err)
	}
	fshots, _ = ListFormatShots(ctx, db, f.ID)
	if fshots[0].Description != "Close up" {
		t.Fatalf("shot order after move wrong: %+v", fshots)
	}

	// Seed a content item: it copies type/mode, outline, and shot-list in order.
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
	shots, _ := ListShots(ctx, db, item.ID)
	if len(shots) != 2 || shots[0].Description != "Close up" || shots[0].Status != "planned" {
		t.Fatalf("seeded shots wrong: %+v", shots)
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
