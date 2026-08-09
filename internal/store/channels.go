package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidChannel is returned when a channel fails validation.
var ErrInvalidChannel = errors.New("channel name is required")

// ErrChannelNotFound is returned when a channel lookup finds no match.
var ErrChannelNotFound = errors.New("channel not found")

// ChannelByName returns the channel with the given name (case-insensitive), or
// ErrChannelNotFound. Used by bulk import to map rows to existing channels.
func ChannelByName(ctx context.Context, db *sql.DB, name string) (Channel, error) {
	name = strings.TrimSpace(name)
	var (
		c            Channel
		created, upd string
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, name, kind, created_at, updated_at FROM channels WHERE name = ? COLLATE NOCASE`, name).
		Scan(&c.ID, &c.Name, &c.Kind, &created, &upd)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrChannelNotFound
	}
	if err != nil {
		return Channel{}, fmt.Errorf("channel by name: %w", err)
	}
	c.CreatedAt = parseTS(created)
	c.UpdatedAt = parseTS(upd)
	return c, nil
}

// Channel is a brand/destination (e.g. a YouTube channel or blog). See
// docs/SPEC.md §3.
type Channel struct {
	ID        int64
	Name      string
	Kind      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateChannel inserts a channel and returns it. Name is required (after
// trimming); kind is optional free text.
func CreateChannel(ctx context.Context, db *sql.DB, name, kind string) (Channel, error) {
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	if name == "" {
		return Channel{}, ErrInvalidChannel
	}

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO channels (name, kind, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		name, kind, ts, ts)
	if err != nil {
		return Channel{}, fmt.Errorf("insert channel: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Channel{}, fmt.Errorf("channel last insert id: %w", err)
	}
	return Channel{ID: id, Name: name, Kind: kind, CreatedAt: now, UpdatedAt: now}, nil
}

// ListChannels returns all channels ordered case-insensitively by name.
func ListChannels(ctx context.Context, db *sql.DB) ([]Channel, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, kind, created_at, updated_at FROM channels ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query channels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Channel
	for rows.Next() {
		var (
			c            Channel
			created, upd string
		)
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &created, &upd); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		c.CreatedAt = parseTS(created)
		c.UpdatedAt = parseTS(upd)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}
	return out, nil
}

// parseTS parses a stored timestamp, tolerating both our RFC3339 writes and
// SQLite's default CURRENT_TIMESTAMP format. Returns the zero time if neither
// layout matches.
func parseTS(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
