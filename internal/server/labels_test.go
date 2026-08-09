package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestLabelAddRemoveAndBoardFilter(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()

	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	a, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	// Beta exists so we can prove the label filter excludes it.
	if _, err := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Beta"); err != nil {
		t.Fatalf("CreateContentItem Beta: %v", err)
	}
	aBase := "/content/" + strconv.FormatInt(a.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, aBase, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add a label (with a color) to Alpha.
	if rec := postForm(t, s, aBase+"/labels", token, url.Values{"name": {"evergreen"}, "color": {"green"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add label = %d, want 303", rec.Code)
	}
	labels, _ := store.ListLabelsForItem(ctx, s.db, a.ID)
	if len(labels) != 1 || labels[0].Color != "green" {
		t.Fatalf("label not added correctly: %+v", labels)
	}
	labelID := labels[0].ID

	// Board (no filter) shows both items and the label chip.
	boardRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(boardRec, httptest.NewRequest(http.MethodGet, "/content", nil))
	body := boardRec.Body.String()
	if !strings.Contains(body, "Alpha") || !strings.Contains(body, "Beta") {
		t.Error("board missing an item")
	}
	if !strings.Contains(body, "label-green") {
		t.Error("board missing label chip")
	}

	// Filtered board shows only Alpha.
	filterRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(filterRec, httptest.NewRequest(http.MethodGet, "/content?label="+strconv.FormatInt(labelID, 10), nil))
	fbody := filterRec.Body.String()
	if !strings.Contains(fbody, "Alpha") || strings.Contains(fbody, "Beta") {
		t.Error("label filter did not restrict to Alpha")
	}

	// Remove the label.
	if rec := postForm(t, s, aBase+"/labels/"+strconv.FormatInt(labelID, 10)+"/remove", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("remove label = %d, want 303", rec.Code)
	}
	labels, _ = store.ListLabelsForItem(ctx, s.db, a.ID)
	if len(labels) != 0 {
		t.Errorf("label not removed: %+v", labels)
	}
}
