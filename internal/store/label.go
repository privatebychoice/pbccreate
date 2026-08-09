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

// ErrInvalidLabel is returned when a label name is empty.
var ErrInvalidLabel = errors.New("label name is required")

// LabelColors is the fixed palette of label color keys (each maps to a CSS
// class, keeping colors out of inline styles for CSP safety).
var LabelColors = []string{"blue", "green", "amber", "red", "purple", "teal", "pink", "gray"}

// Label is an internal organizational label in a channel (SPEC §5.14).
type Label struct {
	ID        int64
	ChannelID int64
	Name      string
	Color     string
}

func normLabelColor(c string) string {
	if slices.Contains(LabelColors, c) {
		return c
	}
	return "blue"
}

// GetOrCreateLabel returns the channel's label with the given name
// (case-insensitive), creating it with the color if absent (existing labels keep
// their color).
func GetOrCreateLabel(ctx context.Context, db *sql.DB, channelID int64, name, color string) (Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Label{}, ErrInvalidLabel
	}
	var l Label
	err := db.QueryRowContext(ctx,
		`SELECT id, channel_id, name, color FROM project_labels WHERE channel_id = ? AND name = ? COLLATE NOCASE`,
		channelID, name).Scan(&l.ID, &l.ChannelID, &l.Name, &l.Color)
	switch {
	case err == nil:
		return l, nil
	case !errors.Is(err, sql.ErrNoRows):
		return Label{}, fmt.Errorf("lookup label: %w", err)
	}

	color = normLabelColor(color)
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO project_labels (channel_id, name, color, created_at) VALUES (?, ?, ?, ?)`,
		channelID, name, color, ts)
	if err != nil {
		return Label{}, fmt.Errorf("insert label: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Label{}, fmt.Errorf("label last insert id: %w", err)
	}
	return Label{ID: id, ChannelID: channelID, Name: name, Color: color}, nil
}

// ListLabelsForChannel returns a channel's labels ordered by name.
func ListLabelsForChannel(ctx context.Context, db *sql.DB, channelID int64) ([]Label, error) {
	return queryLabels(ctx, db,
		`SELECT id, channel_id, name, color FROM project_labels WHERE channel_id = ? ORDER BY name COLLATE NOCASE`, channelID)
}

// ListAllLabels returns every label (across channels) for the board filter.
func ListAllLabels(ctx context.Context, db *sql.DB) ([]Label, error) {
	return queryLabels(ctx, db,
		`SELECT id, channel_id, name, color FROM project_labels ORDER BY name COLLATE NOCASE`)
}

// ListLabelsForItem returns the labels assigned to a content item.
func ListLabelsForItem(ctx context.Context, db *sql.DB, contentItemID int64) ([]Label, error) {
	return queryLabels(ctx, db, `
		SELECT l.id, l.channel_id, l.name, l.color
		FROM content_item_labels cil
		JOIN project_labels l ON l.id = cil.label_id
		WHERE cil.content_item_id = ?
		ORDER BY l.name COLLATE NOCASE`, contentItemID)
}

// LabelsByItem returns a map of content item id -> its labels, for the board.
func LabelsByItem(ctx context.Context, db *sql.DB) (map[int64][]Label, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT cil.content_item_id, l.id, l.channel_id, l.name, l.color
		FROM content_item_labels cil
		JOIN project_labels l ON l.id = cil.label_id
		ORDER BY l.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query labels by item: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64][]Label)
	for rows.Next() {
		var (
			itemID int64
			l      Label
		)
		if err := rows.Scan(&itemID, &l.ID, &l.ChannelID, &l.Name, &l.Color); err != nil {
			return nil, fmt.Errorf("scan label by item: %w", err)
		}
		out[itemID] = append(out[itemID], l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate labels by item: %w", err)
	}
	return out, nil
}

// AssignLabel links a label to a content item (idempotent).
func AssignLabel(ctx context.Context, db *sql.DB, contentItemID, labelID int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO content_item_labels (content_item_id, label_id) VALUES (?, ?)`,
		contentItemID, labelID)
	if err != nil {
		return fmt.Errorf("assign label: %w", err)
	}
	return nil
}

// UnassignLabel removes a label from a content item.
func UnassignLabel(ctx context.Context, db *sql.DB, contentItemID, labelID int64) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM content_item_labels WHERE content_item_id = ? AND label_id = ?`,
		contentItemID, labelID)
	if err != nil {
		return fmt.Errorf("unassign label: %w", err)
	}
	return nil
}

func queryLabels(ctx context.Context, db *sql.DB, query string, args ...any) ([]Label, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query labels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.ChannelID, &l.Name, &l.Color); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate labels: %w", err)
	}
	return out, nil
}
