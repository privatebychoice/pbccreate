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

func TestTakeAddCircleDelete(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	shot, _ := store.AddShot(ctx, s.db, item.ID, store.Shot{Description: "Wide"})
	base := "/content/" + strconv.FormatInt(item.ID, 10)
	shotBase := base + "/shots/" + strconv.FormatInt(shot.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add a circled take with a rating.
	if rec := postForm(t, s, shotBase+"/takes", token, url.Values{
		"label":   {"Take 1"},
		"rating":  {"4"},
		"circled": {"1"},
		"notes":   {"the keeper"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add take = %d, want 303", rec.Code)
	}
	takes, _ := store.ListTakesForShot(ctx, s.db, shot.ID)
	if len(takes) != 1 || !takes[0].Circled || takes[0].Rating != 4 {
		t.Fatalf("take not stored: %+v", takes)
	}
	takeID := takes[0].ID

	// The item page shows the take under its shot.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detail.Body.String(), "Take 1") {
		t.Error("item detail missing the take")
	}

	// Toggle circle off.
	if rec := postForm(t, s, shotBase+"/takes/"+strconv.FormatInt(takeID, 10)+"/circle", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("circle toggle = %d, want 303", rec.Code)
	}
	if after, _ := store.ListTakesForShot(ctx, s.db, shot.ID); after[0].Circled {
		t.Error("take should be un-circled")
	}

	// A take under a shot on a different item 404s (ownership).
	other, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Beta")
	wrong := "/content/" + strconv.FormatInt(other.ID, 10) + "/shots/" + strconv.FormatInt(shot.ID, 10) + "/takes"
	if rec := postForm(t, s, wrong, token, url.Values{"label": {"x"}}); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-item take add = %d, want 404", rec.Code)
	}

	// Delete the take.
	if rec := postForm(t, s, shotBase+"/takes/"+strconv.FormatInt(takeID, 10)+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete take = %d, want 303", rec.Code)
	}
	if takes, _ := store.ListTakesForShot(ctx, s.db, shot.ID); len(takes) != 0 {
		t.Errorf("take not deleted: %d remain", len(takes))
	}
}
