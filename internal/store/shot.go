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

// Shot list errors.
var (
	ErrInvalidShot       = errors.New("shot description is required")
	ErrInvalidShotStatus = errors.New("invalid shot status")
	ErrShotNotFound      = errors.New("shot not found")
)

// ShotStatuses is the shot production vocabulary (SPEC §5.3).
var ShotStatuses = []string{"planned", "shot", "selected"}

// Shot is one ordered entry in a content item's shot list (SPEC §5.3). The
// fields are intentionally generic; different creator modes emphasize different
// columns (framing for single-cam, camera for multi-cam, etc.).
type Shot struct {
	ID            int64
	ContentItemID int64
	Position      int
	Description   string
	Scene         string
	Framing       string
	Camera        string
	Status        string
	Notes         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListShots returns a content item's shots in position order.
func ListShots(ctx context.Context, db *sql.DB, contentItemID int64) ([]Shot, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, content_item_id, position, description, scene, framing, camera, status, notes, created_at, updated_at
		FROM shots
		WHERE content_item_id = ?
		ORDER BY position`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query shots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Shot
	for rows.Next() {
		var (
			s            Shot
			created, upd string
		)
		if err := rows.Scan(&s.ID, &s.ContentItemID, &s.Position, &s.Description, &s.Scene, &s.Framing, &s.Camera, &s.Status, &s.Notes, &created, &upd); err != nil {
			return nil, fmt.Errorf("scan shot: %w", err)
		}
		s.CreatedAt = parseTS(created)
		s.UpdatedAt = parseTS(upd)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shots: %w", err)
	}
	return out, nil
}

// AddShot appends a shot to a content item's list. Description is required; an
// empty status defaults to "planned".
func AddShot(ctx context.Context, db *sql.DB, contentItemID int64, s Shot) (Shot, error) {
	s.Description = strings.TrimSpace(s.Description)
	s.Scene = strings.TrimSpace(s.Scene)
	s.Framing = strings.TrimSpace(s.Framing)
	s.Camera = strings.TrimSpace(s.Camera)
	s.Notes = strings.TrimSpace(s.Notes)
	if s.Description == "" {
		return Shot{}, ErrInvalidShot
	}
	s.Status = strings.TrimSpace(s.Status)
	if s.Status == "" {
		s.Status = "planned"
	} else if !slices.Contains(ShotStatuses, s.Status) {
		return Shot{}, ErrInvalidShotStatus
	}

	var pos int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM shots WHERE content_item_id = ?`,
		contentItemID).Scan(&pos); err != nil {
		return Shot{}, fmt.Errorf("next shot position: %w", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO shots (content_item_id, position, description, scene, framing, camera, status, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		contentItemID, pos, s.Description, s.Scene, s.Framing, s.Camera, s.Status, s.Notes, ts, ts)
	if err != nil {
		return Shot{}, fmt.Errorf("insert shot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Shot{}, fmt.Errorf("shot last insert id: %w", err)
	}
	s.ID = id
	s.ContentItemID = contentItemID
	s.Position = pos
	s.CreatedAt = now
	s.UpdatedAt = now
	return s, nil
}

// UpdateShotStatus sets a shot's status, scoped to its content item.
func UpdateShotStatus(ctx context.Context, db *sql.DB, id, contentItemID int64, status string) error {
	if !slices.Contains(ShotStatuses, status) {
		return ErrInvalidShotStatus
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE shots SET status = ?, updated_at = ? WHERE id = ? AND content_item_id = ?`,
		status, ts, id, contentItemID)
	if err != nil {
		return fmt.Errorf("update shot status: %w", err)
	}
	return checkAffected(res, ErrShotNotFound)
}

// DeleteShot removes a shot scoped to its content item.
func DeleteShot(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM shots WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete shot: %w", err)
	}
	return checkAffected(res, ErrShotNotFound)
}

// MoveShot swaps a shot's position with its neighbor ("up"/"down").
func MoveShot(ctx context.Context, db *sql.DB, id, contentItemID int64, dir string) error {
	err := moveOrdered(ctx, db, "shots", id, contentItemID, dir)
	if errors.Is(err, errOrderedNotFound) {
		return ErrShotNotFound
	}
	return err
}

// checkAffected returns notFound if the result changed no rows.
func checkAffected(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}
