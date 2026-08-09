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

func TestAssetLibraryCreateSearchEdit(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/assets", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Create two assets.
	if rec := postForm(t, s, "/assets", token, url.Values{
		"name": {"City drone shot"}, "kind": {"b_roll"}, "tags": {"urban, aerial"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create asset = %d, want 303", rec.Code)
	}
	if rec := postForm(t, s, "/assets", token, url.Values{
		"name": {"Sunrise Theme"}, "kind": {"music"}, "license": {"royalty-free"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create asset 2 = %d, want 303", rec.Code)
	}
	if all, _ := store.ListLibraryAssets(ctx, s.db, "", ""); len(all) != 2 {
		t.Fatalf("want 2 assets, got %d", len(all))
	}

	// Search by tag returns only the drone shot.
	searchRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(searchRec, httptest.NewRequest(http.MethodGet, "/assets?q=aerial", nil))
	body := searchRec.Body.String()
	if !strings.Contains(body, "City drone shot") || strings.Contains(body, "Sunrise Theme") {
		t.Error("tag search did not filter to the drone shot")
	}

	// Kind filter returns only music.
	kindRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(kindRec, httptest.NewRequest(http.MethodGet, "/assets?kind=music", nil))
	kbody := kindRec.Body.String()
	if !strings.Contains(kbody, "Sunrise Theme") || strings.Contains(kbody, "City drone shot") {
		t.Error("kind filter did not restrict to music")
	}

	// Edit the drone shot.
	assets, _ := store.ListLibraryAssets(ctx, s.db, "aerial", "")
	loc := "/assets/" + strconv.FormatInt(assets[0].ID, 10)
	if rec := postForm(t, s, loc, token, url.Values{
		"name": {"City drone shot"}, "kind": {"b_roll"}, "tags": {"urban, aerial, 4k"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update asset = %d, want 303", rec.Code)
	}
	if got, _ := store.GetLibraryAsset(ctx, s.db, assets[0].ID); !strings.Contains(got.Tags, "4k") {
		t.Fatalf("tags not updated: %+v", got)
	}

	// Blank name update is a 400.
	if rec := postForm(t, s, loc, token, url.Values{"name": {"  "}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name update = %d, want 400", rec.Code)
	}

	// Delete.
	if rec := postForm(t, s, loc+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete asset = %d, want 303", rec.Code)
	}
	if all, _ := store.ListLibraryAssets(ctx, s.db, "", ""); len(all) != 1 {
		t.Errorf("want 1 asset after delete, got %d", len(all))
	}
}
