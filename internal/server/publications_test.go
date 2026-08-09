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

func TestPublicationCreateEditDeleteAndChecklist(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Before any publication with an output file, the checklist output-file check fails.
	if !strings.Contains(getRec.Body.String(), "No publication has an output file recorded yet.") {
		t.Error("expected failing output-file check before any publication")
	}

	// Create a publication with an output file recorded.
	if rec := postForm(t, s, base+"/publications", token, url.Values{
		"platform":    {"YouTube"},
		"output_file": {"/masters/final.mp4"},
		"visibility":  {"public"},
		"posted_on":   {"2026-08-09"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create publication = %d, want 303", rec.Code)
	}
	pubs, _ := store.ListPublications(ctx, s.db, item.ID)
	if len(pubs) != 1 || pubs[0].OutputFile != "/masters/final.mp4" {
		t.Fatalf("publication not stored: %+v", pubs)
	}
	pubID := pubs[0].ID

	// The detail page now reports the output-file check as satisfied.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detail.Body.String(), "Output file recorded on a publication.") {
		t.Error("expected output-file check to pass after recording an output file")
	}

	// Update: change visibility to unlisted.
	if rec := postForm(t, s, base+"/publications/"+strconv.FormatInt(pubID, 10), token, url.Values{
		"platform":   {"YouTube"},
		"visibility": {"unlisted"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update publication = %d, want 303", rec.Code)
	}
	after, _ := store.GetPublication(ctx, s.db, pubID, item.ID)
	if after.Visibility != "unlisted" {
		t.Fatalf("visibility not updated: %+v", after)
	}

	// Delete.
	if rec := postForm(t, s, base+"/publications/"+strconv.FormatInt(pubID, 10)+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete publication = %d, want 303", rec.Code)
	}
	if pubs, _ := store.ListPublications(ctx, s.db, item.ID); len(pubs) != 0 {
		t.Errorf("publication not deleted: %d remain", len(pubs))
	}
}
