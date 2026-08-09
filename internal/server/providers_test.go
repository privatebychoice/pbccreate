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

func TestProviderCreateEditDelete(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/providers", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Create redirects to the new provider's detail page.
	rec := postForm(t, s, "/providers", token, url.Values{
		"name":         {"Storyblocks"},
		"service_type": {"stock"},
		"website_url":  {"https://example.test"},
		"status":       {"active"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create provider = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/providers/") {
		t.Fatalf("redirect = %q, want /providers/<id>", loc)
	}
	id, _ := strconv.ParseInt(strings.TrimPrefix(loc, "/providers/"), 10, 64)

	providers, _ := store.ListAssetProviders(ctx, s.db)
	if len(providers) != 1 || providers[0].ServiceType != "stock" {
		t.Fatalf("provider not stored correctly: %+v", providers)
	}

	// List page shows it.
	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/providers", nil))
	if !strings.Contains(listRec.Body.String(), "Storyblocks") {
		t.Error("providers list missing Storyblocks")
	}

	// Update: change status to lapsed + add a portal URL.
	if rec := postForm(t, s, loc, token, url.Values{
		"name":         {"Storyblocks"},
		"service_type": {"stock"},
		"status":       {"lapsed"},
		"portal_url":   {"https://portal.example.test"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update provider = %d, want 303", rec.Code)
	}
	after, _ := store.GetAssetProvider(ctx, s.db, id)
	if after.Status != "lapsed" || after.PortalURL != "https://portal.example.test" {
		t.Fatalf("update not applied: %+v", after)
	}

	// Empty name is a 400 with the error message.
	if rec := postForm(t, s, loc, token, url.Values{"name": {"  "}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name update = %d, want 400", rec.Code)
	}

	// Delete redirects back to the list.
	if rec := postForm(t, s, loc+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete provider = %d, want 303", rec.Code)
	}
	if _, err := store.GetAssetProvider(ctx, s.db, id); err != store.ErrProviderNotFound {
		t.Errorf("after delete err = %v, want ErrProviderNotFound", err)
	}
}
