package store

import (
	"context"
	"testing"
)

func TestRetrospectiveUpsert(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")

	// No row yet: an empty retrospective is returned (not an error).
	got, err := GetRetrospective(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("GetRetrospective (empty): %v", err)
	}
	if got.HasContent() {
		t.Errorf("fresh retrospective should be empty: %+v", got)
	}

	// First save inserts.
	saved, err := SaveRetrospective(ctx, db, Retrospective{
		ContentItemID: item.ID,
		WhatWorked:    "  strong hook  ",
		ToImprove:     "tighten the middle",
		ReviewedOn:    "2026-08-09",
	})
	if err != nil {
		t.Fatalf("SaveRetrospective (insert): %v", err)
	}
	if saved.WhatWorked != "strong hook" {
		t.Errorf("what_worked = %q, want trimmed", saved.WhatWorked)
	}
	if !saved.HasContent() {
		t.Error("saved retrospective should report content")
	}

	// Second save updates the same row (no duplicate).
	if _, err := SaveRetrospective(ctx, db, Retrospective{
		ContentItemID:    item.ID,
		WhatWorked:       "strong hook",
		PerformanceNotes: "10k views in a week",
	}); err != nil {
		t.Fatalf("SaveRetrospective (update): %v", err)
	}
	after, _ := GetRetrospective(ctx, db, item.ID)
	if after.PerformanceNotes != "10k views in a week" {
		t.Errorf("performance notes not updated: %+v", after)
	}
	// to_improve was cleared on the second save (full-record upsert).
	if after.ToImprove != "" {
		t.Errorf("to_improve = %q, want cleared by upsert", after.ToImprove)
	}

	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM retrospectives WHERE content_item_id = ?`, item.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("row count = %d, want 1 (upsert, not duplicate)", n)
	}
}
