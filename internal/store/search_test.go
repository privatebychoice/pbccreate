package store

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestGlobalSearch(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")

	// Seed matches across several sources, all sharing the token "quantum".
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Quantum computing intro")
	if _, err := SaveScript(ctx, db, item.ID, "Today we explore quantum entanglement.", 150); err != nil {
		t.Fatalf("SaveScript: %v", err)
	}
	_, _ = CreateIdea(ctx, db, Idea{ChannelID: ch.ID, Title: "Quantum myths", Note: "debunk"})
	_, _ = CreateLibraryAsset(ctx, db, LibraryAsset{Name: "Lab b-roll", Tags: "quantum, science", Kind: "b_roll"})
	// A non-matching item, to prove filtering.
	_, _ = CreateContentItem(ctx, db, ch.ID, "video", "", "Cooking basics")

	// Empty query → no results.
	if got, _ := GlobalSearch(ctx, db, "  "); got != nil {
		t.Errorf("empty query should return nil, got %+v", got)
	}

	results, err := GlobalSearch(ctx, db, "quantum")
	if err != nil {
		t.Fatalf("GlobalSearch: %v", err)
	}

	byKind := map[string]SearchResult{}
	for _, r := range results {
		byKind[r.Kind] = r
	}
	for _, kind := range []string{"Content", "Script", "Idea", "Asset"} {
		if _, ok := byKind[kind]; !ok {
			t.Errorf("expected a %s result for 'quantum'; got kinds %v", kind, kinds(results))
		}
	}
	// The content hit links to the item and does not include the non-matching one.
	if c := byKind["Content"]; c.URL != "/content/"+strconv.FormatInt(item.ID, 10) || !strings.Contains(c.Title, "Quantum") {
		t.Errorf("content result wrong: %+v", c)
	}
	// Script snippet carries the matched body.
	if !strings.Contains(byKind["Script"].Snippet, "entanglement") {
		t.Errorf("script snippet missing body: %q", byKind["Script"].Snippet)
	}
	// Case-insensitive.
	if got, _ := GlobalSearch(ctx, db, "QUANTUM"); len(got) != len(results) {
		t.Errorf("search should be case-insensitive: %d vs %d", len(got), len(results))
	}
	// A term that matches nothing.
	if got, _ := GlobalSearch(ctx, db, "zzzznomatch"); len(got) != 0 {
		t.Errorf("no-match query should be empty, got %d", len(got))
	}
}

func kinds(rs []SearchResult) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Kind)
	}
	return out
}
