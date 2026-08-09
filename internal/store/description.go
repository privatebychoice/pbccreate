package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Description is the templated, copy-ready description for a content item, one
// per item (SPEC §5.4). Blocks are stored raw; Render trims and joins them.
type Description struct {
	ContentItemID int64
	Intro         string
	Chapters      string
	Links         string
	Sponsor       string
	Hashtags      string
	Disclosure    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Render assembles the non-empty blocks, in publish order, separated by blank
// lines — the text the operator copies into the upload.
func (d Description) Render() string {
	blocks := []string{d.Intro, d.Chapters, d.Links, d.Sponsor, d.Disclosure, d.Hashtags}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if t := strings.TrimSpace(b); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// GetDescription returns the item's description, or an empty one (not an error)
// when none exists yet, so the editor can render.
func GetDescription(ctx context.Context, db *sql.DB, contentItemID int64) (Description, error) {
	var (
		d            Description
		created, upd string
	)
	err := db.QueryRowContext(ctx,
		`SELECT content_item_id, intro, chapters, links, sponsor, hashtags, disclosure, created_at, updated_at
		 FROM descriptions WHERE content_item_id = ?`, contentItemID).
		Scan(&d.ContentItemID, &d.Intro, &d.Chapters, &d.Links, &d.Sponsor, &d.Hashtags, &d.Disclosure, &created, &upd)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Description{ContentItemID: contentItemID}, nil
	case err != nil:
		return Description{}, fmt.Errorf("get description: %w", err)
	}
	d.CreatedAt = parseTS(created)
	d.UpdatedAt = parseTS(upd)
	return d, nil
}

// SaveDescription upserts the description for a content item and returns it.
func SaveDescription(ctx context.Context, db *sql.DB, d Description) (Description, error) {
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO descriptions (content_item_id, intro, chapters, links, sponsor, hashtags, disclosure, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(content_item_id) DO UPDATE SET
			intro = excluded.intro,
			chapters = excluded.chapters,
			links = excluded.links,
			sponsor = excluded.sponsor,
			hashtags = excluded.hashtags,
			disclosure = excluded.disclosure,
			updated_at = excluded.updated_at`,
		d.ContentItemID, d.Intro, d.Chapters, d.Links, d.Sponsor, d.Hashtags, d.Disclosure, now, now)
	if err != nil {
		return Description{}, fmt.Errorf("save description: %w", err)
	}
	return GetDescription(ctx, db, d.ContentItemID)
}
