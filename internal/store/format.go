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

// Format errors.
var (
	ErrInvalidFormat         = errors.New("format name is required")
	ErrFormatNotFound        = errors.New("format not found")
	ErrInvalidFormatSegment  = errors.New("segment title is required")
	ErrFormatSegmentNotFound = errors.New("format segment not found")
)

// Format is a reusable content template for a channel (SPEC §5.14): a default
// type/mode plus a default outline. Seeding a format creates a content item and
// copies the outline in.
type Format struct {
	ID          int64
	ChannelID   int64
	ChannelName string
	Name        string
	Description string
	DefaultType string
	DefaultMode string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FormatSummary pairs a format with its outline-segment count for the list view.
type FormatSummary struct {
	Format
	SegmentCount int
}

// FormatSegment is one default outline row in a format.
type FormatSegment struct {
	ID            int64
	FormatID      int64
	Position      int
	Title         string
	Notes         string
	TargetSeconds int
}

// normFormatType returns a valid content type, defaulting to "video".
func normFormatType(typ string) string {
	typ = strings.TrimSpace(typ)
	if slices.Contains(ContentTypes, typ) {
		return typ
	}
	return "video"
}

// normFormatMode returns a valid mode for the type, or "" (modes apply only to
// video/short).
func normFormatMode(typ, mode string) string {
	mode = strings.TrimSpace(mode)
	if !modeApplies(typ) || mode == "" || !slices.Contains(CreatorModes, mode) {
		return ""
	}
	return mode
}

// CreateFormat inserts a format (name required) and returns it.
func CreateFormat(ctx context.Context, db *sql.DB, f Format) (Format, error) {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		return Format{}, ErrInvalidFormat
	}
	f.DefaultType = normFormatType(f.DefaultType)
	f.DefaultMode = normFormatMode(f.DefaultType, f.DefaultMode)
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO formats (channel_id, name, description, default_type, default_mode, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ChannelID, f.Name, strings.TrimSpace(f.Description), f.DefaultType, f.DefaultMode, ts, ts)
	if err != nil {
		return Format{}, fmt.Errorf("insert format: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Format{}, fmt.Errorf("format last insert id: %w", err)
	}
	return GetFormat(ctx, db, id)
}

// GetFormat returns one format with its channel name, or ErrFormatNotFound.
func GetFormat(ctx context.Context, db *sql.DB, id int64) (Format, error) {
	var (
		f            Format
		created, upd string
	)
	err := db.QueryRowContext(ctx, `
		SELECT f.id, f.channel_id, COALESCE(c.name, ''), f.name, f.description, f.default_type, f.default_mode, f.created_at, f.updated_at
		FROM formats f LEFT JOIN channels c ON c.id = f.channel_id
		WHERE f.id = ?`, id).Scan(&f.ID, &f.ChannelID, &f.ChannelName, &f.Name, &f.Description, &f.DefaultType, &f.DefaultMode, &created, &upd)
	if errors.Is(err, sql.ErrNoRows) {
		return Format{}, ErrFormatNotFound
	}
	if err != nil {
		return Format{}, fmt.Errorf("get format: %w", err)
	}
	f.CreatedAt = parseTS(created)
	f.UpdatedAt = parseTS(upd)
	return f, nil
}

// ListFormats returns all formats with segment counts, ordered by channel then
// name.
func ListFormats(ctx context.Context, db *sql.DB) ([]FormatSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT f.id, f.channel_id, COALESCE(c.name, ''), f.name, f.description, f.default_type, f.default_mode, f.created_at, f.updated_at,
			(SELECT COUNT(*) FROM format_outline_segments fos WHERE fos.format_id = f.id)
		FROM formats f LEFT JOIN channels c ON c.id = f.channel_id
		ORDER BY c.name COLLATE NOCASE, f.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query formats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []FormatSummary
	for rows.Next() {
		var (
			fs           FormatSummary
			created, upd string
		)
		if err := rows.Scan(&fs.ID, &fs.ChannelID, &fs.ChannelName, &fs.Name, &fs.Description,
			&fs.DefaultType, &fs.DefaultMode, &created, &upd, &fs.SegmentCount); err != nil {
			return nil, fmt.Errorf("scan format: %w", err)
		}
		fs.CreatedAt = parseTS(created)
		fs.UpdatedAt = parseTS(upd)
		out = append(out, fs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate formats: %w", err)
	}
	return out, nil
}

// UpdateFormat updates a format's fields (name required).
func UpdateFormat(ctx context.Context, db *sql.DB, f Format) error {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		return ErrInvalidFormat
	}
	f.DefaultType = normFormatType(f.DefaultType)
	f.DefaultMode = normFormatMode(f.DefaultType, f.DefaultMode)
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE formats SET name = ?, description = ?, default_type = ?, default_mode = ?, updated_at = ? WHERE id = ?`,
		f.Name, strings.TrimSpace(f.Description), f.DefaultType, f.DefaultMode, ts, f.ID)
	if err != nil {
		return fmt.Errorf("update format: %w", err)
	}
	return checkAffected(res, ErrFormatNotFound)
}

// DeleteFormat removes a format (and its default outline, by cascade).
func DeleteFormat(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM formats WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete format: %w", err)
	}
	return checkAffected(res, ErrFormatNotFound)
}

// ListFormatSegments returns a format's default outline in order.
func ListFormatSegments(ctx context.Context, db *sql.DB, formatID int64) ([]FormatSegment, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, format_id, position, title, notes, target_seconds FROM format_outline_segments
		 WHERE format_id = ? ORDER BY position, id`, formatID)
	if err != nil {
		return nil, fmt.Errorf("query format segments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []FormatSegment
	for rows.Next() {
		var fs FormatSegment
		if err := rows.Scan(&fs.ID, &fs.FormatID, &fs.Position, &fs.Title, &fs.Notes, &fs.TargetSeconds); err != nil {
			return nil, fmt.Errorf("scan format segment: %w", err)
		}
		out = append(out, fs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate format segments: %w", err)
	}
	return out, nil
}

// AddFormatSegment appends a default outline row (title required).
func AddFormatSegment(ctx context.Context, db *sql.DB, formatID int64, title, notes string, targetSeconds int) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrInvalidFormatSegment
	}
	var next int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM format_outline_segments WHERE format_id = ?`, formatID).Scan(&next); err != nil {
		return fmt.Errorf("next format segment position: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO format_outline_segments (format_id, position, title, notes, target_seconds) VALUES (?, ?, ?, ?, ?)`,
		formatID, next, title, strings.TrimSpace(notes), targetSeconds); err != nil {
		return fmt.Errorf("insert format segment: %w", err)
	}
	return nil
}

// DeleteFormatSegment removes a default outline row scoped to its format.
func DeleteFormatSegment(ctx context.Context, db *sql.DB, segID, formatID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM format_outline_segments WHERE id = ? AND format_id = ?`, segID, formatID)
	if err != nil {
		return fmt.Errorf("delete format segment: %w", err)
	}
	return checkAffected(res, ErrFormatSegmentNotFound)
}

// MoveFormatSegment reorders a default outline row up or down.
func MoveFormatSegment(ctx context.Context, db *sql.DB, segID, formatID int64, dir string) error {
	err := moveOrdered(ctx, db, "format_outline_segments", segID, formatID, dir)
	if errors.Is(err, errOrderedNotFound) {
		return ErrFormatSegmentNotFound
	}
	return err
}

// SeedContentItemFromFormat creates a content item from a format (using its
// default type/mode) and copies the format's default outline into it. The title
// is supplied by the caller.
func SeedContentItemFromFormat(ctx context.Context, db *sql.DB, formatID int64, title string) (ContentItem, error) {
	f, err := GetFormat(ctx, db, formatID)
	if err != nil {
		return ContentItem{}, err
	}
	item, err := CreateContentItem(ctx, db, f.ChannelID, f.DefaultType, f.DefaultMode, title)
	if err != nil {
		return ContentItem{}, err
	}
	segs, err := ListFormatSegments(ctx, db, formatID)
	if err != nil {
		return ContentItem{}, err
	}
	for _, s := range segs {
		if _, err := AddOutlineSegment(ctx, db, item.ID, s.Title, s.Notes, s.TargetSeconds); err != nil {
			return ContentItem{}, fmt.Errorf("seed outline: %w", err)
		}
	}
	return item, nil
}
