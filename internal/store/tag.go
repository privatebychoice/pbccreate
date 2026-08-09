package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidTag is returned when a tag name is empty.
var ErrInvalidTag = errors.New("tag name is required")

// Tag is an outward-facing SEO keyword in a channel's library (SPEC §5.10).
type Tag struct {
	ID        int64
	ChannelID int64
	Name      string
}

// GetOrCreateTag returns the channel's tag with the given name (case-insensitive),
// creating it if absent.
func GetOrCreateTag(ctx context.Context, db *sql.DB, channelID int64, name string) (Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Tag{}, ErrInvalidTag
	}
	// Existing (case-insensitive)?
	var t Tag
	err := db.QueryRowContext(ctx,
		`SELECT id, channel_id, name FROM tags WHERE channel_id = ? AND name = ? COLLATE NOCASE`,
		channelID, name).Scan(&t.ID, &t.ChannelID, &t.Name)
	switch {
	case err == nil:
		return t, nil
	case !errors.Is(err, sql.ErrNoRows):
		return Tag{}, fmt.Errorf("lookup tag: %w", err)
	}

	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO tags (channel_id, name, created_at) VALUES (?, ?, ?)`, channelID, name, ts)
	if err != nil {
		return Tag{}, fmt.Errorf("insert tag: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Tag{}, fmt.Errorf("tag last insert id: %w", err)
	}
	return Tag{ID: id, ChannelID: channelID, Name: name}, nil
}

// ListTagsForChannel returns a channel's tag library ordered by name.
func ListTagsForChannel(ctx context.Context, db *sql.DB, channelID int64) ([]Tag, error) {
	return queryTags(ctx, db,
		`SELECT id, channel_id, name FROM tags WHERE channel_id = ? ORDER BY name COLLATE NOCASE`, channelID)
}

// ListTagsForItem returns the tags assigned to a content item, ordered by name.
func ListTagsForItem(ctx context.Context, db *sql.DB, contentItemID int64) ([]Tag, error) {
	return queryTags(ctx, db, `
		SELECT t.id, t.channel_id, t.name
		FROM content_item_tags cit
		JOIN tags t ON t.id = cit.tag_id
		WHERE cit.content_item_id = ?
		ORDER BY t.name COLLATE NOCASE`, contentItemID)
}

// AssignTag links a tag to a content item (idempotent).
func AssignTag(ctx context.Context, db *sql.DB, contentItemID, tagID int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO content_item_tags (content_item_id, tag_id) VALUES (?, ?)`,
		contentItemID, tagID)
	if err != nil {
		return fmt.Errorf("assign tag: %w", err)
	}
	return nil
}

// UnassignTag removes a tag from a content item.
func UnassignTag(ctx context.Context, db *sql.DB, contentItemID, tagID int64) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM content_item_tags WHERE content_item_id = ? AND tag_id = ?`,
		contentItemID, tagID)
	if err != nil {
		return fmt.Errorf("unassign tag: %w", err)
	}
	return nil
}

func queryTags(ctx context.Context, db *sql.DB, query string, args ...any) ([]Tag, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.ChannelID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return out, nil
}
