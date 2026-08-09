package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ErrInvalidContentItem is returned when a content item fails validation.
var ErrInvalidContentItem = errors.New("content item requires a channel, a title, and a valid type")

// Pipeline vocabulary (see docs/SPEC.md §3, §4). ContentStatuses is in board
// order.
var (
	ContentStatuses = []string{"idea", "scripting", "shooting", "editing", "scheduled", "published", "archived"}
	ContentTypes    = []string{"video", "short", "blog", "social"}
	CreatorModes    = []string{"faceless", "single_cam", "multi_cam", "obs"}
)

// ContentItem is the central unit of planning (see docs/SPEC.md §3).
type ContentItem struct {
	ID          int64
	ChannelID   int64
	ChannelName string // populated by list joins; empty otherwise
	Type        string
	Mode        string
	Title       string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// modeApplies reports whether a creator mode is meaningful for the given type.
func modeApplies(typ string) bool { return typ == "video" || typ == "short" }

// CreateContentItem inserts a content item at status "idea" and returns it. The
// creator mode is cleared for types where it does not apply (blog, social).
func CreateContentItem(ctx context.Context, db *sql.DB, channelID int64, typ, mode, title string) (ContentItem, error) {
	typ = strings.TrimSpace(typ)
	mode = strings.TrimSpace(mode)
	title = strings.TrimSpace(title)

	if channelID <= 0 || title == "" || !slices.Contains(ContentTypes, typ) {
		return ContentItem{}, ErrInvalidContentItem
	}
	if !modeApplies(typ) {
		mode = ""
	} else if mode != "" && !slices.Contains(CreatorModes, mode) {
		return ContentItem{}, ErrInvalidContentItem
	}

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO content_items (channel_id, type, mode, title, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'idea', ?, ?)`,
		channelID, typ, mode, title, ts, ts)
	if err != nil {
		return ContentItem{}, fmt.Errorf("insert content item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ContentItem{}, fmt.Errorf("content item last insert id: %w", err)
	}
	return ContentItem{
		ID: id, ChannelID: channelID, Type: typ, Mode: mode,
		Title: title, Status: "idea", CreatedAt: now, UpdatedAt: now,
	}, nil
}

// ListContentItems returns all content items (with channel name) ordered
// case-insensitively by title.
func ListContentItems(ctx context.Context, db *sql.DB) ([]ContentItem, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ci.id, ci.channel_id, c.name, ci.type, ci.mode, ci.title, ci.status, ci.created_at, ci.updated_at
		FROM content_items ci
		JOIN channels c ON c.id = ci.channel_id
		ORDER BY ci.title COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query content items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ContentItem
	for rows.Next() {
		var (
			ci           ContentItem
			created, upd string
		)
		if err := rows.Scan(&ci.ID, &ci.ChannelID, &ci.ChannelName, &ci.Type, &ci.Mode, &ci.Title, &ci.Status, &created, &upd); err != nil {
			return nil, fmt.Errorf("scan content item: %w", err)
		}
		ci.CreatedAt = parseTS(created)
		ci.UpdatedAt = parseTS(upd)
		out = append(out, ci)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content items: %w", err)
	}
	return out, nil
}
