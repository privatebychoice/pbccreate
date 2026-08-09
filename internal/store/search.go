package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// SearchResult is one hit from a global search (SPEC §5.19).
type SearchResult struct {
	Kind    string // human label of the source, e.g. "Script", "Idea"
	Title   string // the owning item/entity name
	Snippet string // a short excerpt of the matched text
	URL     string // in-app link to the match
}

// searchCapPerKind bounds how many matches each source contributes, so a broad
// query cannot produce an unbounded page. Kinds that hit the cap are noted in the
// UI text, not silently truncated.
const searchCapPerKind = 50

// GlobalSearch runs a case-insensitive substring search across scripts, ideas,
// notes, and metadata, returning results grouped by source (in a fixed order).
// An empty query returns no results.
func GlobalSearch(ctx context.Context, db *sql.DB, query string) ([]SearchResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	like := "%" + q + "%"

	var out []SearchResult
	// each source: (kind label, url builder from the scanned id, SQL selecting
	// id, title, snippet). The single LIKE bind is reused for every column via a
	// concatenation in the WHERE/snippet expression.
	sources := []struct {
		kind string
		url  func(id int64) string
		sql  string
	}{
		{"Content", contentURL, `
			SELECT id, title, title FROM content_items
			WHERE title LIKE ? ORDER BY title COLLATE NOCASE LIMIT ?`},
		{"Script", contentURL, `
			SELECT s.content_item_id, ci.title, s.body
			FROM scripts s JOIN content_items ci ON ci.id = s.content_item_id
			WHERE s.body LIKE ? ORDER BY ci.title COLLATE NOCASE LIMIT ?`},
		{"Idea", ideaURL, `
			SELECT id, title, title || ' ' || note FROM ideas
			WHERE title LIKE ? OR note LIKE ? ORDER BY title COLLATE NOCASE LIMIT ?`},
		{"Description", contentURL, `
			SELECT d.content_item_id, ci.title,
				d.intro || ' ' || d.chapters || ' ' || d.links || ' ' || d.sponsor || ' ' || d.credits || ' ' || d.disclosure || ' ' || d.hashtags
			FROM descriptions d JOIN content_items ci ON ci.id = d.content_item_id
			WHERE (d.intro || ' ' || d.chapters || ' ' || d.links || ' ' || d.sponsor || ' ' || d.credits || ' ' || d.disclosure || ' ' || d.hashtags) LIKE ?
			ORDER BY ci.title COLLATE NOCASE LIMIT ?`},
		{"Outline", contentURL, `
			SELECT o.content_item_id, ci.title, o.title || ' ' || o.notes
			FROM outline_segments o JOIN content_items ci ON ci.id = o.content_item_id
			WHERE o.title LIKE ? OR o.notes LIKE ? ORDER BY ci.title COLLATE NOCASE LIMIT ?`},
		{"Shot", contentURL, `
			SELECT sh.content_item_id, ci.title, sh.description || ' ' || sh.notes
			FROM shots sh JOIN content_items ci ON ci.id = sh.content_item_id
			WHERE sh.description LIKE ? OR sh.notes LIKE ? ORDER BY ci.title COLLATE NOCASE LIMIT ?`},
		{"Retrospective", contentURL, `
			SELECT r.content_item_id, ci.title, r.what_worked || ' ' || r.to_improve || ' ' || r.performance_notes
			FROM retrospectives r JOIN content_items ci ON ci.id = r.content_item_id
			WHERE (r.what_worked || ' ' || r.to_improve || ' ' || r.performance_notes) LIKE ?
			ORDER BY ci.title COLLATE NOCASE LIMIT ?`},
		{"Music cue", contentURL, `
			SELECT c.content_item_id, ci.title, c.title || ' ' || c.artist || ' ' || c.notes
			FROM music_cues c JOIN content_items ci ON ci.id = c.content_item_id
			WHERE c.title LIKE ? OR c.artist LIKE ? OR c.notes LIKE ? ORDER BY ci.title COLLATE NOCASE LIMIT ?`},
		{"Asset", assetURL, `
			SELECT id, name, name || ' ' || tags || ' ' || notes FROM asset_library
			WHERE name LIKE ? OR tags LIKE ? OR notes LIKE ? ORDER BY name COLLATE NOCASE LIMIT ?`},
	}

	for _, src := range sources {
		// Bind the LIKE pattern once per '?' placeholder that precedes the LIMIT.
		n := strings.Count(src.sql, "?") - 1
		args := make([]any, 0, n+1)
		for i := 0; i < n; i++ {
			args = append(args, like)
		}
		args = append(args, searchCapPerKind)

		if err := searchInto(ctx, db, &out, src.kind, src.url, src.sql, args...); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// searchInto runs one source query and appends its rows to out.
func searchInto(ctx context.Context, db *sql.DB, out *[]SearchResult, kind string, url func(int64) string, sqlStr string, args ...any) error {
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("search %s: %w", kind, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			id            int64
			title, source string
		)
		if err := rows.Scan(&id, &title, &source); err != nil {
			return fmt.Errorf("scan %s result: %w", kind, err)
		}
		*out = append(*out, SearchResult{Kind: kind, Title: title, Snippet: snippet(source), URL: url(id)})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s results: %w", kind, err)
	}
	return nil
}

func contentURL(id int64) string { return "/content/" + strconv.FormatInt(id, 10) }
func ideaURL(id int64) string    { return "/ideas/" + strconv.FormatInt(id, 10) }
func assetURL(id int64) string   { return "/assets/" + strconv.FormatInt(id, 10) }

// snippet collapses whitespace and truncates to a bounded, rune-safe excerpt.
func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 140 {
		return string(r[:140]) + "…"
	}
	return s
}
