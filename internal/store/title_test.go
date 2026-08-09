package store

import (
	"context"
	"testing"
)

func TestTitleCandidatesAndSwipe(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Working title")

	if err := AddTitleCandidate(ctx, db, item.ID, "  "); err != ErrInvalidTitleCandidate {
		t.Errorf("empty candidate err = %v, want ErrInvalidTitleCandidate", err)
	}
	_ = AddTitleCandidate(ctx, db, item.ID, "How I built X")
	_ = AddTitleCandidate(ctx, db, item.ID, "The truth about X")
	cands, _ := ListTitleCandidates(ctx, db, item.ID)
	if len(cands) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(cands))
	}

	// Choose the second; exactly one chosen, and it sorts first.
	if err := ChooseTitleCandidate(ctx, db, cands[1].ID, item.ID); err != nil {
		t.Fatalf("ChooseTitleCandidate: %v", err)
	}
	cands, _ = ListTitleCandidates(ctx, db, item.ID)
	if !cands[0].Chosen || cands[0].Text != "The truth about X" {
		t.Fatalf("chosen ordering wrong: %+v", cands)
	}
	if cands[1].Chosen {
		t.Errorf("second candidate should not be chosen: %+v", cands[1])
	}

	// Choosing another switches the mark (still exactly one chosen).
	if err := ChooseTitleCandidate(ctx, db, cands[1].ID, item.ID); err != nil {
		t.Fatalf("re-choose: %v", err)
	}
	cands, _ = ListTitleCandidates(ctx, db, item.ID)
	chosenCount := 0
	for _, c := range cands {
		if c.Chosen {
			chosenCount++
		}
	}
	if chosenCount != 1 {
		t.Fatalf("want exactly 1 chosen, got %d", chosenCount)
	}

	// Choosing a candidate from another item is not found.
	other, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Other")
	if err := ChooseTitleCandidate(ctx, db, cands[0].ID, other.ID); err != ErrTitleCandidateNotFound {
		t.Errorf("cross-item choose err = %v, want ErrTitleCandidateNotFound", err)
	}

	if err := DeleteTitleCandidate(ctx, db, cands[0].ID, item.ID); err != nil {
		t.Fatalf("DeleteTitleCandidate: %v", err)
	}
	if cands, _ := ListTitleCandidates(ctx, db, item.ID); len(cands) != 1 {
		t.Fatalf("want 1 candidate after delete, got %d", len(cands))
	}

	// Swipe file (channel-scoped).
	if err := AddSwipe(ctx, db, ch.ID, "  ", ""); err != ErrInvalidSwipe {
		t.Errorf("empty swipe err = %v, want ErrInvalidSwipe", err)
	}
	_ = AddSwipe(ctx, db, ch.ID, "How I X in Y", "worked well")
	sw, _ := ListSwipe(ctx, db, ch.ID)
	if len(sw) != 1 || sw[0].Pattern != "How I X in Y" {
		t.Fatalf("swipe wrong: %+v", sw)
	}
	if err := DeleteSwipe(ctx, db, sw[0].ID, ch.ID); err != nil {
		t.Fatalf("DeleteSwipe: %v", err)
	}
	if sw, _ := ListSwipe(ctx, db, ch.ID); len(sw) != 0 {
		t.Errorf("swipe not deleted: %d remain", len(sw))
	}
}
