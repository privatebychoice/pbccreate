package store

import (
	"context"
	"testing"
)

func TestSeriesCRUDEpisodesAndOrder(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	other, _ := CreateChannel(ctx, db, "PBC", "youtube")

	if _, err := CreateSeries(ctx, db, ch.ID, "  ", ""); err != ErrInvalidSeries {
		t.Errorf("empty name err = %v, want ErrInvalidSeries", err)
	}

	sr, err := CreateSeries(ctx, db, ch.ID, "VPN Deep Dive", "multi-part")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if sr.ChannelName != "TUL" {
		t.Errorf("channel name = %q, want TUL", sr.ChannelName)
	}

	a, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Ep1")
	b, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Ep2")
	foreign, _ := CreateContentItem(ctx, db, other.ID, "video", "", "Wrong channel")

	// Cross-channel add is rejected.
	if err := AddEpisode(ctx, db, sr.ID, foreign.ID); err != ErrItemChannelDiffer {
		t.Errorf("cross-channel add err = %v, want ErrItemChannelDiffer", err)
	}

	if err := AddEpisode(ctx, db, sr.ID, a.ID); err != nil {
		t.Fatalf("AddEpisode a: %v", err)
	}
	if err := AddEpisode(ctx, db, sr.ID, b.ID); err != nil {
		t.Fatalf("AddEpisode b: %v", err)
	}
	// Duplicate add is rejected.
	if err := AddEpisode(ctx, db, sr.ID, a.ID); err != ErrEpisodeExists {
		t.Errorf("dup add err = %v, want ErrEpisodeExists", err)
	}

	eps, _ := ListEpisodes(ctx, db, sr.ID)
	if len(eps) != 2 || eps[0].Title != "Ep1" || eps[1].Title != "Ep2" {
		t.Fatalf("episode order wrong: %+v", eps)
	}

	// Move Ep2 up → it leads.
	if err := MoveEpisode(ctx, db, eps[1].ID, sr.ID, "up"); err != nil {
		t.Fatalf("MoveEpisode: %v", err)
	}
	eps, _ = ListEpisodes(ctx, db, sr.ID)
	if eps[0].Title != "Ep2" || eps[1].Title != "Ep1" {
		t.Fatalf("order after move wrong: %+v", eps)
	}

	// Arc notes.
	if err := UpdateEpisodeArc(ctx, db, eps[0].ID, sr.ID, "picks up from the cold open"); err != nil {
		t.Fatalf("UpdateEpisodeArc: %v", err)
	}
	eps, _ = ListEpisodes(ctx, db, sr.ID)
	if eps[0].ArcNotes != "picks up from the cold open" {
		t.Errorf("arc notes not saved: %+v", eps[0])
	}

	// Done vs planned: publish Ep1, expect 1 done of 2.
	if err := UpdateContentItemStatus(ctx, db, a.ID, "published"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	list, _ := ListSeries(ctx, db)
	if len(list) != 1 || list[0].EpisodeCount != 2 || list[0].DoneCount != 1 {
		t.Fatalf("series summary wrong: %+v", list)
	}

	// Remove an episode (item survives).
	if err := RemoveEpisode(ctx, db, eps[0].ID, sr.ID); err != nil {
		t.Fatalf("RemoveEpisode: %v", err)
	}
	if eps, _ := ListEpisodes(ctx, db, sr.ID); len(eps) != 1 {
		t.Fatalf("want 1 episode after remove, got %d", len(eps))
	}
	if _, err := GetContentItem(ctx, db, b.ID); err != nil {
		t.Errorf("content item should survive episode removal: %v", err)
	}

	// Delete series cascades episode links.
	if err := DeleteSeries(ctx, db, sr.ID); err != nil {
		t.Fatalf("DeleteSeries: %v", err)
	}
	if _, err := GetSeries(ctx, db, sr.ID); err != ErrSeriesNotFound {
		t.Errorf("after delete err = %v, want ErrSeriesNotFound", err)
	}
}
