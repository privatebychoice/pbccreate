package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Outline segment errors.
var (
	ErrInvalidSegment         = errors.New("segment title is required")
	ErrOutlineSegmentNotFound = errors.New("outline segment not found")
	ErrInvalidMove            = errors.New("invalid move direction")
)

// OutlineSegment is one ordered beat/section of a content item's outline
// (SPEC §5.2). TargetSeconds is 0 when unset.
type OutlineSegment struct {
	ID            int64
	ContentItemID int64
	Position      int
	Title         string
	Notes         string
	TargetSeconds int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListOutlineSegments returns a content item's segments in position order.
func ListOutlineSegments(ctx context.Context, db *sql.DB, contentItemID int64) ([]OutlineSegment, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, content_item_id, position, title, notes, target_seconds, created_at, updated_at
		FROM outline_segments
		WHERE content_item_id = ?
		ORDER BY position`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query outline segments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []OutlineSegment
	for rows.Next() {
		var (
			s            OutlineSegment
			target       sql.NullInt64
			created, upd string
		)
		if err := rows.Scan(&s.ID, &s.ContentItemID, &s.Position, &s.Title, &s.Notes, &target, &created, &upd); err != nil {
			return nil, fmt.Errorf("scan outline segment: %w", err)
		}
		if target.Valid {
			s.TargetSeconds = int(target.Int64)
		}
		s.CreatedAt = parseTS(created)
		s.UpdatedAt = parseTS(upd)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outline segments: %w", err)
	}
	return out, nil
}

// AddOutlineSegment appends a segment at the end of the item's outline. Title is
// required; targetSeconds <= 0 is stored as NULL (unset).
func AddOutlineSegment(ctx context.Context, db *sql.DB, contentItemID int64, title, notes string, targetSeconds int) (OutlineSegment, error) {
	title = strings.TrimSpace(title)
	notes = strings.TrimSpace(notes)
	if title == "" {
		return OutlineSegment{}, ErrInvalidSegment
	}

	var pos int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM outline_segments WHERE content_item_id = ?`,
		contentItemID).Scan(&pos); err != nil {
		return OutlineSegment{}, fmt.Errorf("next segment position: %w", err)
	}

	var target any
	if targetSeconds > 0 {
		target = targetSeconds
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO outline_segments (content_item_id, position, title, notes, target_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		contentItemID, pos, title, notes, target, ts, ts)
	if err != nil {
		return OutlineSegment{}, fmt.Errorf("insert outline segment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return OutlineSegment{}, fmt.Errorf("segment last insert id: %w", err)
	}

	seg := OutlineSegment{
		ID: id, ContentItemID: contentItemID, Position: pos,
		Title: title, Notes: notes, CreatedAt: now, UpdatedAt: now,
	}
	if targetSeconds > 0 {
		seg.TargetSeconds = targetSeconds
	}
	return seg, nil
}

// UpdateOutlineSegment edits a segment's title, notes, and target seconds,
// scoped to its content item. Title is required; targetSeconds <= 0 clears the
// target (NULL).
func UpdateOutlineSegment(ctx context.Context, db *sql.DB, id, contentItemID int64, title, notes string, targetSeconds int) error {
	title = strings.TrimSpace(title)
	notes = strings.TrimSpace(notes)
	if title == "" {
		return ErrInvalidSegment
	}
	var target any
	if targetSeconds > 0 {
		target = targetSeconds
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE outline_segments
		SET title = ?, notes = ?, target_seconds = ?, updated_at = ?
		WHERE id = ? AND content_item_id = ?`,
		title, notes, target, ts, id, contentItemID)
	if err != nil {
		return fmt.Errorf("update outline segment: %w", err)
	}
	return checkAffected(res, ErrOutlineSegmentNotFound)
}

// OutlineSegmentExists reports whether a segment with the given id belongs to the
// content item (used to validate shot->beat links).
func OutlineSegmentExists(ctx context.Context, db *sql.DB, id, contentItemID int64) (bool, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM outline_segments WHERE id = ? AND content_item_id = ?`,
		id, contentItemID).Scan(&n); err != nil {
		return false, fmt.Errorf("outline segment exists: %w", err)
	}
	return n > 0, nil
}

// DeleteOutlineSegment removes a segment scoped to its content item. Returns
// ErrOutlineSegmentNotFound if no row matched.
func DeleteOutlineSegment(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM outline_segments WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete outline segment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("segment rows affected: %w", err)
	}
	if n == 0 {
		return ErrOutlineSegmentNotFound
	}
	return nil
}

// MoveOutlineSegment swaps a segment's position with its neighbor in the given
// direction ("up" or "down"). Moving past an edge is a no-op.
func MoveOutlineSegment(ctx context.Context, db *sql.DB, id, contentItemID int64, dir string) error {
	err := moveOrdered(ctx, db, "outline_segments", id, contentItemID, dir)
	if errors.Is(err, errOrderedNotFound) {
		return ErrOutlineSegmentNotFound
	}
	return err
}
