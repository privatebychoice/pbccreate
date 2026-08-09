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

func TestMusicCueAddToAttributionAndDelete(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	prov, _ := store.CreateAssetProvider(ctx, s.db, store.AssetProvider{Name: "Artlist", ServiceType: "music"})
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add a cue linked to the provider.
	if rec := postForm(t, s, base+"/music-cues", token, url.Values{
		"title":       {"Sunrise"},
		"artist":      {"Artist"},
		"in_point":    {"0:00"},
		"out_point":   {"1:23"},
		"license":     {"royalty-free"},
		"provider_id": {strconv.FormatInt(prov.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add cue = %d, want 303", rec.Code)
	}
	cues, _ := store.ListMusicCues(ctx, s.db, item.ID)
	if len(cues) != 1 || cues[0].Title != "Sunrise" {
		t.Fatalf("cue not stored: %+v", cues)
	}
	cueID := cues[0].ID

	// The item page shows the cue sheet.
	detail := httptest.NewRecorder()
	s.Handler().ServeHTTP(detail, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detail.Body.String(), "Sunrise") {
		t.Error("item detail missing the cue")
	}

	// Turn the cue into a music attribution; it carries provider/license and is
	// marked for the credits block.
	if rec := postForm(t, s, base+"/music-cues/"+strconv.FormatInt(cueID, 10)+"/attribution", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("cue to attribution = %d, want 303", rec.Code)
	}
	attrs, _ := store.ListAttributions(ctx, s.db, item.ID)
	if len(attrs) != 1 {
		t.Fatalf("want 1 attribution, got %d", len(attrs))
	}
	a := attrs[0]
	if a.Kind != "music" || a.Name != "Sunrise" || a.License != "royalty-free" || a.ProviderID != prov.ID || !a.IncludedInDescription {
		t.Fatalf("attribution from cue wrong: %+v", a)
	}

	// Delete the cue.
	if rec := postForm(t, s, base+"/music-cues/"+strconv.FormatInt(cueID, 10)+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete cue = %d, want 303", rec.Code)
	}
	if cues, _ := store.ListMusicCues(ctx, s.db, item.ID); len(cues) != 0 {
		t.Errorf("cue not deleted: %d remain", len(cues))
	}
}
