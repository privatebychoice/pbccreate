package store

import (
	"context"
	"testing"
)

func TestIdeaCRUDScoreAndPromote(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")

	if _, err := CreateIdea(ctx, db, Idea{ChannelID: ch.ID, Title: "  "}); err != ErrInvalidIdea {
		t.Errorf("empty title err = %v, want ErrInvalidIdea", err)
	}

	// Fully scored idea; factors clamp to 0..10.
	high, err := CreateIdea(ctx, db, Idea{
		ChannelID: ch.ID, Title: "High", Impact: 9, Confidence: 8, Effort: 2, Source: "  a comment ",
	})
	if err != nil {
		t.Fatalf("CreateIdea high: %v", err)
	}
	if high.Source != "a comment" {
		t.Errorf("source = %q, want trimmed", high.Source)
	}
	if got := high.Score(); got != 36 { // 9*8/2
		t.Errorf("score = %v, want 36", got)
	}

	// Effort over the cap clamps to 10; partial scoring => 0.
	over, _ := CreateIdea(ctx, db, Idea{ChannelID: ch.ID, Title: "Over", Impact: 5, Confidence: 5, Effort: 99})
	full, _ := GetIdea(ctx, db, over.ID)
	if full.Effort != 10 {
		t.Errorf("effort = %d, want clamped to 10", full.Effort)
	}
	unscored, _ := CreateIdea(ctx, db, Idea{ChannelID: ch.ID, Title: "Unscored", Impact: 5})
	if unscored.Score() != 0 {
		t.Errorf("partial score = %v, want 0", unscored.Score())
	}

	// List is sorted by score desc: High (36) before Over (2.5) before Unscored (0).
	list, err := ListIdeas(ctx, db)
	if err != nil || len(list) != 3 {
		t.Fatalf("ListIdeas: %v (n=%d)", err, len(list))
	}
	if list[0].Title != "High" || list[2].Title != "Unscored" {
		t.Fatalf("sort order wrong: %s, %s, %s", list[0].Title, list[1].Title, list[2].Title)
	}
	if list[0].ChannelName != "TUL" {
		t.Errorf("channel name join = %q, want TUL", list[0].ChannelName)
	}

	// Update editable fields.
	high.Note = "great hook"
	high.Status = "parked"
	if err := UpdateIdea(ctx, db, high); err != nil {
		t.Fatalf("UpdateIdea: %v", err)
	}
	after, _ := GetIdea(ctx, db, high.ID)
	if after.Note != "great hook" || after.Status != "parked" {
		t.Fatalf("update not applied: %+v", after)
	}

	// Promote: link to a content item and flip status.
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "High")
	if err := MarkIdeaPromoted(ctx, db, high.ID, item.ID); err != nil {
		t.Fatalf("MarkIdeaPromoted: %v", err)
	}
	promoted, _ := GetIdea(ctx, db, high.ID)
	if promoted.Status != "promoted" || promoted.PromotedContentItemID != item.ID {
		t.Fatalf("promotion not recorded: %+v", promoted)
	}

	if err := DeleteIdea(ctx, db, over.ID); err != nil {
		t.Fatalf("DeleteIdea: %v", err)
	}
	if _, err := GetIdea(ctx, db, over.ID); err != ErrIdeaNotFound {
		t.Errorf("after delete err = %v, want ErrIdeaNotFound", err)
	}
}
