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

func TestAttributionAddToggleAndCreditsFill(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()

	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Alpha")
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add an attribution with explicit required credit text, included.
	if rec := postForm(t, s, base+"/attributions", token, url.Values{
		"name":        {"Sunrise"},
		"kind":        {"music"},
		"provider":    {"Artist"},
		"license":     {"CC-BY"},
		"credit_text": {"Sunrise by Artist (CC-BY)"},
		"included":    {"1"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add attribution = %d, want 303", rec.Code)
	}
	// Add a second, auto-line attribution, NOT included.
	if rec := postForm(t, s, base+"/attributions", token, url.Values{
		"name":     {"Skyline"},
		"kind":     {"stock"},
		"provider": {"Studio"},
		"license":  {"royalty-free"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("add attribution 2 = %d, want 303", rec.Code)
	}

	attrs, _ := store.ListAttributions(ctx, s.db, item.ID)
	if len(attrs) != 2 {
		t.Fatalf("want 2 attributions, got %d", len(attrs))
	}
	// Newest first: Skyline is [0] and defaults to not included (checkbox unset).
	if attrs[0].Name != "Skyline" || attrs[0].IncludedInDescription {
		t.Fatalf("attr[0] unexpected: %+v", attrs[0])
	}
	skylineID := attrs[0].ID

	// Fill credits: only the included Sunrise line should appear.
	if rec := postForm(t, s, base+"/description/credits", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("fill credits = %d, want 303", rec.Code)
	}
	desc, _ := store.GetDescription(ctx, s.db, item.ID)
	if !strings.Contains(desc.Credits, "Sunrise by Artist (CC-BY)") {
		t.Errorf("credits missing Sunrise line: %q", desc.Credits)
	}
	if strings.Contains(desc.Credits, "Skyline") {
		t.Errorf("credits should exclude not-included Skyline: %q", desc.Credits)
	}
	// Credits feed into the rendered description.
	if !strings.Contains(desc.Render(), "Sunrise by Artist (CC-BY)") {
		t.Errorf("rendered description missing credits block: %q", desc.Render())
	}

	// Toggle Skyline into the credits, refill: now the auto line appears.
	if rec := postForm(t, s, base+"/attributions/"+strconv.FormatInt(skylineID, 10)+"/toggle", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("toggle attribution = %d, want 303", rec.Code)
	}
	if rec := postForm(t, s, base+"/description/credits", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("refill credits = %d, want 303", rec.Code)
	}
	desc, _ = store.GetDescription(ctx, s.db, item.ID)
	if !strings.Contains(desc.Credits, "Skyline by Studio (royalty-free)") {
		t.Errorf("credits missing auto Skyline line after toggle: %q", desc.Credits)
	}

	// Delete Skyline.
	if rec := postForm(t, s, base+"/attributions/"+strconv.FormatInt(skylineID, 10)+"/delete", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete attribution = %d, want 303", rec.Code)
	}
	attrs, _ = store.ListAttributions(ctx, s.db, item.ID)
	if len(attrs) != 1 || attrs[0].Name != "Sunrise" {
		t.Fatalf("after delete want only Sunrise, got %+v", attrs)
	}
}
