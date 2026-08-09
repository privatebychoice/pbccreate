package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrNotBlog is returned when a blog-only operation targets a non-blog item.
var ErrNotBlog = errors.New("content item is not a blog")

// DerivedBlogID returns the id of the blog item derived from the given source
// (0 if none). A source has at most one derived blog in this v1 flow.
func DerivedBlogID(ctx context.Context, db *sql.DB, sourceID int64) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM content_items WHERE derived_from_id = ? AND type = 'blog' ORDER BY id LIMIT 1`, sourceID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("derived blog id: %w", err)
	}
	return id, nil
}

// DerivedSourceID returns the source item an item was derived from (0 if none).
func DerivedSourceID(ctx context.Context, db *sql.DB, itemID int64) (int64, error) {
	var src sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT derived_from_id FROM content_items WHERE id = ?`, itemID).Scan(&src)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrContentItemNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("derived source id: %w", err)
	}
	if src.Valid {
		return src.Int64, nil
	}
	return 0, nil
}

// RepurposeToBlog forks a source item into a new derived blog ContentItem in the
// same channel and seeds it (SPEC §5.9). Returns the new blog item's id. It does
// not guard against creating a second blog — callers redirect to an existing one.
func RepurposeToBlog(ctx context.Context, db *sql.DB, sourceID int64) (int64, error) {
	src, err := GetContentItem(ctx, db, sourceID)
	if err != nil {
		return 0, err
	}
	blog, err := CreateContentItem(ctx, db, src.ChannelID, "blog", "", src.Title)
	if err != nil {
		return 0, fmt.Errorf("create derived blog: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE content_items SET derived_from_id = ? WHERE id = ?`, sourceID, blog.ID); err != nil {
		return 0, fmt.Errorf("link derived blog: %w", err)
	}
	if err := SeedBlog(ctx, db, blog.ID, sourceID); err != nil {
		return 0, err
	}
	return blog.ID, nil
}

// SeedBlog (re)generates a blog's body, links, and tags from its source. It
// OVERWRITES the blog's script body and description links — callers warn first on
// a re-seed.
func SeedBlog(ctx context.Context, db *sql.DB, blogID, sourceID int64) error {
	blog, err := GetContentItem(ctx, db, blogID)
	if err != nil {
		return fmt.Errorf("seed: blog item: %w", err)
	}
	script, err := GetScript(ctx, db, sourceID)
	if err != nil {
		return fmt.Errorf("seed: source script: %w", err)
	}
	segments, err := ListOutlineSegments(ctx, db, sourceID)
	if err != nil {
		return fmt.Errorf("seed: source outline: %w", err)
	}
	if _, err := SaveScript(ctx, db, blogID, buildBlogBody(script.Body, segments), script.WPM); err != nil {
		return fmt.Errorf("seed: save blog body: %w", err)
	}

	srcDesc, err := GetDescription(ctx, db, sourceID)
	if err != nil {
		return fmt.Errorf("seed: source description: %w", err)
	}
	blogDesc, err := GetDescription(ctx, db, blogID)
	if err != nil {
		return fmt.Errorf("seed: blog description: %w", err)
	}
	blogDesc.Intro = srcDesc.Intro
	blogDesc.Links = srcDesc.Links
	if _, err := SaveDescription(ctx, db, blogDesc); err != nil {
		return fmt.Errorf("seed: save blog description: %w", err)
	}

	tags, err := ListTagsForItem(ctx, db, sourceID)
	if err != nil {
		return fmt.Errorf("seed: source tags: %w", err)
	}
	for _, t := range tags {
		bt, err := GetOrCreateTag(ctx, db, blog.ChannelID, t.Name)
		if err != nil {
			return fmt.Errorf("seed: blog tag: %w", err)
		}
		if err := AssignTag(ctx, db, blogID, bt.ID); err != nil {
			return fmt.Errorf("seed: assign blog tag: %w", err)
		}
	}
	return nil
}

// buildBlogBody assembles a Markdown draft: the script prose as the lead, then
// each outline segment as an H2 heading with its notes as an editorial-hint HTML
// comment (not published text).
func buildBlogBody(scriptBody string, segments []OutlineSegment) string {
	var b strings.Builder
	if t := strings.TrimSpace(scriptBody); t != "" {
		b.WriteString(t)
		b.WriteString("\n\n")
	}
	for _, s := range segments {
		fmt.Fprintf(&b, "## %s\n\n", s.Title)
		if n := strings.TrimSpace(s.Notes); n != "" {
			fmt.Fprintf(&b, "<!-- %s -->\n\n", n)
		}
	}
	return strings.TrimSpace(b.String())
}

// RenderBlogMarkdown assembles a portable Markdown document with YAML front
// matter for a blog item (SPEC §5.9). Pure text assembly — no network.
func RenderBlogMarkdown(ctx context.Context, db *sql.DB, blogID int64) (string, error) {
	item, err := GetContentItem(ctx, db, blogID)
	if err != nil {
		return "", err
	}
	if item.Type != "blog" {
		return "", ErrNotBlog
	}
	script, err := GetScript(ctx, db, blogID)
	if err != nil {
		return "", fmt.Errorf("blog markdown: script: %w", err)
	}
	desc, err := GetDescription(ctx, db, blogID)
	if err != nil {
		return "", fmt.Errorf("blog markdown: description: %w", err)
	}
	tags, err := ListTagsForItem(ctx, db, blogID)
	if err != nil {
		return "", fmt.Errorf("blog markdown: tags: %w", err)
	}
	sourceID, err := DerivedSourceID(ctx, db, blogID)
	if err != nil {
		return "", err
	}
	sourceTitle := ""
	if sourceID > 0 {
		if src, err := GetContentItem(ctx, db, sourceID); err == nil {
			sourceTitle = src.Title
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", item.Title)
	fmt.Fprintf(&b, "date: %q\n", item.CreatedAt.Format("2006-01-02"))
	b.WriteString("draft: true\n")
	if len(tags) > 0 {
		names := make([]string, len(tags))
		for i, t := range tags {
			names[i] = fmt.Sprintf("%q", t.Name)
		}
		fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(names, ", "))
	}
	if sourceTitle != "" {
		fmt.Fprintf(&b, "source: %q\n", sourceTitle)
	}
	b.WriteString("---\n\n")

	if body := strings.TrimSpace(script.Body); body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}
	if links := strings.TrimSpace(desc.Links); links != "" {
		b.WriteString("\n## Links / further reading\n\n")
		b.WriteString(links)
		b.WriteString("\n")
	}
	return b.String(), nil
}
