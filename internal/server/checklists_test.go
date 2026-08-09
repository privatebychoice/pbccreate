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

func TestChecklistDefineAndRunOnItem(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	itemBase := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/checklists", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Define a template with two items.
	rec := postForm(t, s, "/checklists", token, url.Values{"name": {"Shoot day"}, "stage": {"shoot_day"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create template = %d, want 303", rec.Code)
	}
	tplLoc := rec.Header().Get("Location")
	tplID, _ := strconv.ParseInt(strings.TrimPrefix(tplLoc, "/checklists/"), 10, 64)
	for _, txt := range []string{"Charge batteries", "Check audio"} {
		if rec := postForm(t, s, tplLoc+"/items", token, url.Values{"text": {txt}}); rec.Code != http.StatusSeeOther {
			t.Fatalf("add item = %d, want 303", rec.Code)
		}
	}

	// Start a run on the content item.
	if rec := postForm(t, s, itemBase+"/checklist-runs", token, url.Values{"template_id": {strconv.FormatInt(tplID, 10)}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("start run = %d, want 303", rec.Code)
	}
	runs, _ := store.ListRuns(ctx, s.db, item.ID)
	if len(runs) != 1 || runs[0].Name != "Shoot day" {
		t.Fatalf("run not created: %+v", runs)
	}
	runID := runs[0].ID
	runItems, _ := store.ListRunItems(ctx, s.db, runID)
	if len(runItems) != 2 {
		t.Fatalf("want 2 run items, got %d", len(runItems))
	}

	// The item page shows the run and its progress.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, itemBase, nil))
	if !strings.Contains(detail.Body.String(), "Charge batteries") {
		t.Error("item detail missing checklist run items")
	}

	// Toggle one item done.
	toggleURL := itemBase + "/checklist-runs/" + strconv.FormatInt(runID, 10) + "/items/" + strconv.FormatInt(runItems[0].ID, 10) + "/toggle"
	if rec := postForm(t, s, toggleURL, token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("toggle run item = %d, want 303", rec.Code)
	}
	after, _ := store.ListRunItems(ctx, s.db, runID)
	if !after[0].Done {
		t.Errorf("run item not toggled done: %+v", after[0])
	}

	// A toggle for a run that is not on this item is a 404 (scoping).
	other, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Beta")
	otherToggle := "/content/" + strconv.FormatInt(other.ID, 10) + "/checklist-runs/" + strconv.FormatInt(runID, 10) + "/items/" + strconv.FormatInt(runItems[0].ID, 10) + "/toggle"
	if rec := postForm(t, s, otherToggle, token, url.Values{}); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-item toggle = %d, want 404", rec.Code)
	}

	// Delete the run.
	if rec := postForm(t, s, itemBase+"/checklist-runs/"+strconv.FormatInt(runID, 10)+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete run = %d, want 303", rec.Code)
	}
	if runs, _ := store.ListRuns(ctx, s.db, item.ID); len(runs) != 0 {
		t.Errorf("run not deleted: %d remain", len(runs))
	}
}
