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

func TestShotAddAndStatusFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add a shot.
	if rec := postForm(t, s, base+"/shots", token, url.Values{
		"description": {"Wide establishing shot"},
		"camera":      {"A"},
		"framing":     {"WS"},
		"status":      {"planned"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add shot = %d, want 303", rec.Code)
	}

	// Detail shows it.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detRec.Body.String(), "Wide establishing shot") {
		t.Error("shot not shown on detail")
	}

	// Update its status to "selected".
	shots, err := store.ListShots(context.Background(), s.db, item.ID)
	if err != nil || len(shots) != 1 {
		t.Fatalf("ListShots: %v (len=%d)", err, len(shots))
	}
	statusPath := base + "/shots/" + strconv.FormatInt(shots[0].ID, 10) + "/status"
	if rec := postForm(t, s, statusPath, token, url.Values{"status": {"selected"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("set status = %d, want 303", rec.Code)
	}
	shots, _ = store.ListShots(context.Background(), s.db, item.ID)
	if shots[0].Status != "selected" {
		t.Errorf("status = %q, want selected", shots[0].Status)
	}
}

func TestShotEditBeatLinkAndLabel(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	beat1, _ := store.AddOutlineSegment(ctx, s.db, item.ID, "The Hook", "", 15)
	beat2, _ := store.AddOutlineSegment(ctx, s.db, item.ID, "The Close", "", 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	// The add form offers a Beat select.
	if !strings.Contains(getRec.Body.String(), "Beat 1 — The Hook") {
		t.Error("beat option not offered in shot form")
	}

	// Add two shots linked to beat 1 → labelled 1A, 1B.
	for _, d := range []string{"phone screen", "card tap"} {
		if rec := postForm(t, s, base+"/shots", token, url.Values{
			"description":        {d},
			"outline_segment_id": {strconv.FormatInt(beat1.ID, 10)},
		}); rec.Code != http.StatusSeeOther {
			t.Fatalf("add shot %q = %d, want 303", d, rec.Code)
		}
	}

	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	body := detRec.Body.String()
	if !strings.Contains(body, "1A · phone screen") || !strings.Contains(body, "1B · card tap") {
		t.Error("beat labels 1A/1B not rendered")
	}

	// Edit the first shot: change description + relink to beat 2 → becomes 2A.
	shots, _ := store.ListShots(ctx, s.db, item.ID)
	updPath := base + "/shots/" + strconv.FormatInt(shots[0].ID, 10) + "/update"
	if rec := postForm(t, s, updPath, token, url.Values{
		"description":        {"phone lock screen"},
		"outline_segment_id": {strconv.FormatInt(beat2.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update shot = %d, want 303", rec.Code)
	}
	shots, _ = store.ListShots(ctx, s.db, item.ID)
	var edited store.Shot
	for _, sh := range shots {
		if sh.ID == shots[0].ID {
			edited = sh
		}
	}
	if edited.Description != "phone lock screen" || edited.OutlineSegmentID != beat2.ID {
		t.Fatalf("edited shot = %+v", edited)
	}

	// A bogus beat on update is a 400.
	if rec := postForm(t, s, updPath, token, url.Values{
		"description":        {"x"},
		"outline_segment_id": {"999999"},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bogus beat update = %d, want 400", rec.Code)
	}
}

func TestOutlineListsLinkedShots(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	beat, _ := store.AddOutlineSegment(ctx, s.db, item.ID, "The Hook", "", 15)
	sh1, _ := store.AddShot(ctx, s.db, item.ID, store.Shot{Description: "phone screen", OutlineSegmentID: beat.ID})
	_, _ = store.AddShot(ctx, s.db, item.ID, store.Shot{Description: "card tap", OutlineSegmentID: beat.ID})
	// An unlinked shot must not appear under any beat.
	_, _ = store.AddShot(ctx, s.db, item.ID, store.Shot{Description: "loose clip"})

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, base, nil))
	body := rec.Body.String()

	// The outline beat shows jump-links to its shots...
	if !strings.Contains(body, `href="#shot-`+strconv.FormatInt(sh1.ID, 10)+`"`) {
		t.Error("outline missing jump-link to linked shot")
	}
	if !strings.Contains(body, "Jump to shot 1A") {
		t.Error("outline missing labelled jump-link 1A")
	}
	// ...and each shot has the matching anchor id target.
	if !strings.Contains(body, `id="shot-`+strconv.FormatInt(sh1.ID, 10)+`"`) {
		t.Error("shot list missing anchor id for jump target")
	}
}

func TestOutlineSegmentEdit(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)
	seg, _ := store.AddOutlineSegment(ctx, s.db, item.ID, "Hook", "old", 15)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	// The renamed label is present.
	if !strings.Contains(getRec.Body.String(), "Segment/Beat Title") {
		t.Error("expected 'Segment/Beat Title' label")
	}

	updPath := base + "/outline/" + strconv.FormatInt(seg.ID, 10) + "/update"
	if rec := postForm(t, s, updPath, token, url.Values{
		"title":          {"The Hook"},
		"notes":          {"new notes"},
		"target_seconds": {"25"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update segment = %d, want 303", rec.Code)
	}
	segs, _ := store.ListOutlineSegments(ctx, s.db, item.ID)
	if segs[0].Title != "The Hook" || segs[0].Notes != "new notes" || segs[0].TargetSeconds != 25 {
		t.Fatalf("edited segment = %+v", segs[0])
	}
}

func TestShotDeleteNotFound(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/shots/999999/delete", token, url.Values{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing shot = %d, want 404", rec.Code)
	}
}
