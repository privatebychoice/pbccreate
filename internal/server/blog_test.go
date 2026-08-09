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

func TestRepurposeAndBlogExport(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	video, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "", "Best VPN 2026")
	_, _ = store.SaveScript(ctx, s.db, video.ID, "A VPN protects your traffic.", 150)
	_, _ = store.AddOutlineSegment(ctx, s.db, video.ID, "What is a VPN", "", 30)
	base := "/content/" + strconv.FormatInt(video.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// The video page offers "Repurpose to blog".
	if !strings.Contains(getRec.Body.String(), "Repurpose to blog") {
		t.Error("video detail missing repurpose action")
	}

	// Repurpose → redirect to the new blog item.
	rec := postForm(t, s, base+"/repurpose", token, url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("repurpose = %d, want 303", rec.Code)
	}
	blogLoc := rec.Header().Get("Location")
	blogID, _ := strconv.ParseInt(strings.TrimPrefix(blogLoc, "/content/"), 10, 64)
	blog, _ := store.GetContentItem(ctx, s.db, blogID)
	if blog.Type != "blog" {
		t.Fatalf("derived item type = %q, want blog", blog.Type)
	}

	// Repurposing again jumps to the existing blog (no second one made).
	rec2 := postForm(t, s, base+"/repurpose", token, url.Values{})
	if rec2.Header().Get("Location") != blogLoc {
		t.Errorf("second repurpose = %q, want existing %q", rec2.Header().Get("Location"), blogLoc)
	}
	if id, _ := store.DerivedBlogID(ctx, s.db, video.ID); id != blogID {
		t.Errorf("a second blog was created")
	}

	// The source page now links to the derived blog.
	srcRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(srcRec, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(srcRec.Body.String(), "open the blog") {
		t.Error("source page missing derived-blog link")
	}

	// The blog page offers Markdown download + re-seed.
	blogRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(blogRec, httptest.NewRequest(http.MethodGet, blogLoc, nil))
	bbody := blogRec.Body.String()
	if !strings.Contains(bbody, "Download Markdown") || !strings.Contains(bbody, "Re-seed from source") {
		t.Error("blog page missing export/reseed actions")
	}

	// Markdown export downloads as an attachment.
	dl := httptest.NewRecorder()
	s.Handler().ServeHTTP(dl, httptest.NewRequest(http.MethodGet, blogLoc+"/blog.md", nil))
	if dl.Code != http.StatusOK {
		t.Fatalf("export = %d, want 200", dl.Code)
	}
	if ct := dl.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("content-type = %q, want text/markdown", ct)
	}
	if cd := dl.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".md") {
		t.Errorf("content-disposition = %q", cd)
	}
	if !strings.Contains(dl.Body.String(), "## What is a VPN") {
		t.Error("exported markdown missing seeded heading")
	}

	// Exporting a non-blog item is a 400.
	badDL := httptest.NewRecorder()
	s.Handler().ServeHTTP(badDL, httptest.NewRequest(http.MethodGet, base+"/blog.md", nil))
	if badDL.Code != http.StatusBadRequest {
		t.Fatalf("export of video = %d, want 400", badDL.Code)
	}
}
