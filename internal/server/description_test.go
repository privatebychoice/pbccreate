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

func TestFormatTimestamp(t *testing.T) {
	cases := map[int]string{0: "0:00", 5: "0:05", 90: "1:30", 3600: "1:00:00", 3723: "1:02:03"}
	for in, want := range cases {
		if got := formatTimestamp(in); got != want {
			t.Errorf("formatTimestamp(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDescriptionSaveFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/description", token, url.Values{
		"intro":      {"Welcome back to the channel."},
		"links":      {"https://example.com"},
		"hashtags":   {"#privacy"},
		"disclosure": {"Not sponsored."},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save description = %d, want 303", rec.Code)
	}

	// The rendered preview reflects the saved blocks.
	detRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(detRec, httptest.NewRequest(http.MethodGet, base, nil))
	body := detRec.Body.String()
	for _, want := range []string{"Welcome back to the channel.", "https://example.com", "#privacy", "Not sponsored."} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered description missing %q", want)
		}
	}
}

func TestDescriptionChaptersFromOutline(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	ctx := context.Background()

	// Two segments: Hook (0s, 15s target) then Body (starts at 0:15).
	if _, err := store.AddOutlineSegment(ctx, s.db, item.ID, "Hook", "", 15); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddOutlineSegment(ctx, s.db, item.ID, "Body", "", 120); err != nil {
		t.Fatal(err)
	}

	base := "/content/" + strconv.FormatInt(item.ID, 10)
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	rec := postForm(t, s, base+"/description/chapters", token, url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("fill chapters = %d, want 303", rec.Code)
	}

	d, err := store.GetDescription(ctx, s.db, item.ID)
	if err != nil {
		t.Fatalf("GetDescription: %v", err)
	}
	want := "0:00 Hook\n0:15 Body"
	if d.Chapters != want {
		t.Errorf("Chapters =\n%q\nwant\n%q", d.Chapters, want)
	}
}
