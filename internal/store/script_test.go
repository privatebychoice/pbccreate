package store

import (
	"context"
	"strings"
	"testing"
)

func TestGetScriptDefaultsWhenMissing(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "VO Test")

	sc, err := GetScript(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if sc.Body != "" {
		t.Errorf("Body = %q, want empty", sc.Body)
	}
	if sc.WPM != DefaultWPM {
		t.Errorf("WPM = %d, want %d", sc.WPM, DefaultWPM)
	}
}

func TestSaveAndGetScript(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "faceless", "VO Test")

	saved, err := SaveScript(ctx, db, item.ID, "one two three four", 120)
	if err != nil {
		t.Fatalf("SaveScript: %v", err)
	}
	if saved.WordCount() != 4 {
		t.Errorf("WordCount = %d, want 4", saved.WordCount())
	}
	// 4 words / 120 wpm * 60 = 2s.
	if saved.EstimatedSeconds() != 2 {
		t.Errorf("EstimatedSeconds = %d, want 2", saved.EstimatedSeconds())
	}

	got, err := GetScript(ctx, db, item.ID)
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if got.Body != "one two three four" || got.WPM != 120 {
		t.Errorf("unexpected script: %+v", got)
	}
}

func TestSaveScriptUpsertAndWPMDefault(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Video")

	if _, err := SaveScript(ctx, db, item.ID, "first version", 100); err != nil {
		t.Fatalf("SaveScript: %v", err)
	}
	// Second save updates in place (upsert) and clamps a bad WPM to the default.
	saved, err := SaveScript(ctx, db, item.ID, "second version body", 0)
	if err != nil {
		t.Fatalf("SaveScript (update): %v", err)
	}
	if saved.Body != "second version body" {
		t.Errorf("Body = %q, want updated", saved.Body)
	}
	if saved.WPM != DefaultWPM {
		t.Errorf("WPM = %d, want default %d", saved.WPM, DefaultWPM)
	}

	// Exactly one row for the item.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM scripts WHERE content_item_id = ?`, item.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("script rows = %d, want 1 (upsert)", count)
	}
}

func TestScriptDurationLabel(t *testing.T) {
	cases := []struct {
		body string
		wpm  int
		want string
	}{
		{"", 150, "0s"},
		{"one two three four five", 150, "2s"},        // 5/150*60 = 2s
		{strings.Repeat("word ", 300), 150, "2m 00s"}, // 300/150*60 = 120s
	}
	for _, tc := range cases {
		s := Script{Body: tc.body, WPM: tc.wpm}
		if got := s.DurationLabel(); got != tc.want {
			t.Errorf("DurationLabel(%q, %d) = %q, want %q", tc.body, tc.wpm, got, tc.want)
		}
	}
}
