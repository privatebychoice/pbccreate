package store

import (
	"context"
	"strings"
	"testing"
)

func TestRepurposeSeedAndMarkdown(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	video, _ := CreateContentItem(ctx, db, ch.ID, "video", "single_cam", "Best VPN 2026")

	// Seed the source video with script, outline, description links, and a tag.
	_, _ = SaveScript(ctx, db, video.ID, "A VPN protects your traffic.", 150)
	_, _ = AddOutlineSegment(ctx, db, video.ID, "What is a VPN", "define it", 30)
	_, _ = AddOutlineSegment(ctx, db, video.ID, "How to choose", "criteria", 90)
	srcDesc, _ := GetDescription(ctx, db, video.ID)
	srcDesc.Links = "https://example.test/guide"
	_, _ = SaveDescription(ctx, db, srcDesc)
	tag, _ := GetOrCreateTag(ctx, db, ch.ID, "privacy")
	_ = AssignTag(ctx, db, video.ID, tag.ID)

	// No derived blog yet.
	if id, _ := DerivedBlogID(ctx, db, video.ID); id != 0 {
		t.Fatalf("expected no derived blog, got %d", id)
	}

	blogID, err := RepurposeToBlog(ctx, db, video.ID)
	if err != nil {
		t.Fatalf("RepurposeToBlog: %v", err)
	}
	blog, _ := GetContentItem(ctx, db, blogID)
	if blog.Type != "blog" || blog.ChannelID != ch.ID || blog.Title != "Best VPN 2026" {
		t.Fatalf("derived blog wrong: %+v", blog)
	}
	if src, _ := DerivedSourceID(ctx, db, blogID); src != video.ID {
		t.Fatalf("derived source = %d, want %d", src, video.ID)
	}
	if id, _ := DerivedBlogID(ctx, db, video.ID); id != blogID {
		t.Fatalf("DerivedBlogID = %d, want %d", id, blogID)
	}

	// The blog body (its script) has the prose lead and outline headings.
	body, _ := GetScript(ctx, db, blogID)
	if !strings.Contains(body.Body, "A VPN protects your traffic.") {
		t.Errorf("blog body missing script prose: %q", body.Body)
	}
	if !strings.Contains(body.Body, "## What is a VPN") || !strings.Contains(body.Body, "## How to choose") {
		t.Errorf("blog body missing outline headings: %q", body.Body)
	}
	if !strings.Contains(body.Body, "<!-- define it -->") {
		t.Errorf("blog body missing editorial-hint comment: %q", body.Body)
	}
	// Links and tags copied.
	bd, _ := GetDescription(ctx, db, blogID)
	if bd.Links != "https://example.test/guide" {
		t.Errorf("blog links not copied: %q", bd.Links)
	}
	if tags, _ := ListTagsForItem(ctx, db, blogID); len(tags) != 1 || tags[0].Name != "privacy" {
		t.Errorf("blog tags not copied: %+v", tags)
	}

	// Markdown export: front matter + body + links block + source reference.
	md, err := RenderBlogMarkdown(ctx, db, blogID)
	if err != nil {
		t.Fatalf("RenderBlogMarkdown: %v", err)
	}
	for _, want := range []string{
		"---", `title: "Best VPN 2026"`, "draft: true", `tags: ["privacy"]`,
		`source: "Best VPN 2026"`, "## What is a VPN", "## Links / further reading",
		"https://example.test/guide",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}

	// Re-seed after an edit overwrites the body back to the seeded draft.
	_, _ = SaveScript(ctx, db, blogID, "my own edited body", 150)
	if err := SeedBlog(ctx, db, blogID, video.ID); err != nil {
		t.Fatalf("SeedBlog re-seed: %v", err)
	}
	reseeded, _ := GetScript(ctx, db, blogID)
	if strings.Contains(reseeded.Body, "my own edited body") {
		t.Errorf("re-seed should overwrite edits: %q", reseeded.Body)
	}

	// Markdown export refuses a non-blog item.
	if _, err := RenderBlogMarkdown(ctx, db, video.ID); err != ErrNotBlog {
		t.Errorf("export of video err = %v, want ErrNotBlog", err)
	}
}
