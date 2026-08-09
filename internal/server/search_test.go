package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestSearchPage(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Quantum computing intro")
	_, _ = store.SaveScript(ctx, s.db, item.ID, "Today we explore quantum entanglement.", 150)
	_, _ = store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Cooking basics")

	// A query with matches renders grouped rows linking to the item.
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/search?q=quantum", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Quantum computing intro") {
		t.Error("search page missing the content title match")
	}
	if !strings.Contains(body, "/content/"+strconv.FormatInt(item.ID, 10)) {
		t.Error("search result missing link to the item")
	}
	if !strings.Contains(body, "entanglement") {
		t.Error("search page missing the script snippet")
	}
	if strings.Contains(body, "Cooking basics") {
		t.Error("non-matching item should not appear")
	}

	// A no-match query shows the empty state.
	empty := httptest.NewRecorder()
	s.Handler().ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/search?q=zzzznomatch", nil))
	if !strings.Contains(empty.Body.String(), "No matches.") {
		t.Error("expected empty-state message for a no-match query")
	}

	// No query: the form renders without a results section.
	none := httptest.NewRecorder()
	s.Handler().ServeHTTP(none, httptest.NewRequest(http.MethodGet, "/search", nil))
	if none.Code != http.StatusOK {
		t.Fatalf("empty search = %d, want 200", none.Code)
	}
}
