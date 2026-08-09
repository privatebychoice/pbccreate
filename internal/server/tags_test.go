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

func TestTagAddRemoveAndHashtags(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Add two tags (get-or-create + assign).
	for _, name := range []string{"privacy tips", "VPN"} {
		if rec := postForm(t, s, base+"/tags", token, url.Values{"name": {name}}); rec.Code != http.StatusSeeOther {
			t.Fatalf("add tag %q = %d, want 303", name, rec.Code)
		}
	}
	tags, _ := store.ListTagsForItem(context.Background(), s.db, item.ID)
	if len(tags) != 2 {
		t.Fatalf("assigned tags = %d, want 2", len(tags))
	}

	// Detail shows the chips + copyable list.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(detRec.Body.String(), "privacy tips") {
		t.Error("detail missing tag chip")
	}

	// Fill hashtags from tags: "privacy tips" -> "#privacytips".
	if rec := postForm(t, s, base+"/description/hashtags", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("fill hashtags = %d, want 303", rec.Code)
	}
	d, _ := store.GetDescription(context.Background(), s.db, item.ID)
	if !strings.Contains(d.Hashtags, "#privacytips") || !strings.Contains(d.Hashtags, "#VPN") {
		t.Errorf("hashtags = %q, want #privacytips and #VPN", d.Hashtags)
	}

	// Remove one tag.
	var vpnID int64
	for _, tg := range tags {
		if tg.Name == "VPN" {
			vpnID = tg.ID
		}
	}
	if rec := postForm(t, s, base+"/tags/"+strconv.FormatInt(vpnID, 10)+"/remove", token, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("remove tag = %d, want 303", rec.Code)
	}
	tags, _ = store.ListTagsForItem(context.Background(), s.db, item.ID)
	if len(tags) != 1 || tags[0].Name != "privacy tips" {
		t.Errorf("after remove: %+v", tags)
	}
}
