package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Thumbnail errors.
var (
	ErrInvalidThumbnail  = errors.New("thumbnail name is required")
	ErrThumbnailNotFound = errors.New("thumbnail not found")
)

// Thumbnail is a saved thumbnail design for a content item; CanvasJSON holds the
// layer model rendered by the thumbnail package (SPEC §5.5).
type Thumbnail struct {
	ID            int64
	ContentItemID int64
	Name          string
	CanvasJSON    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateThumbnail inserts a thumbnail with the given name and initial canvas.
func CreateThumbnail(ctx context.Context, db *sql.DB, contentItemID int64, name, canvasJSON string) (Thumbnail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Thumbnail{}, ErrInvalidThumbnail
	}
	if canvasJSON == "" {
		canvasJSON = "{}"
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO thumbnails (content_item_id, name, canvas_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		contentItemID, name, canvasJSON, ts, ts)
	if err != nil {
		return Thumbnail{}, fmt.Errorf("insert thumbnail: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Thumbnail{}, fmt.Errorf("thumbnail last insert id: %w", err)
	}
	return Thumbnail{ID: id, ContentItemID: contentItemID, Name: name, CanvasJSON: canvasJSON, CreatedAt: now, UpdatedAt: now}, nil
}

// ListThumbnails returns a content item's thumbnails, newest first.
func ListThumbnails(ctx context.Context, db *sql.DB, contentItemID int64) ([]Thumbnail, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, content_item_id, name, canvas_json, created_at, updated_at
		 FROM thumbnails WHERE content_item_id = ? ORDER BY created_at DESC, id DESC`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query thumbnails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Thumbnail
	for rows.Next() {
		t, err := scanThumbnail(rows)
		if err != nil {
			return nil, fmt.Errorf("scan thumbnail: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thumbnails: %w", err)
	}
	return out, nil
}

// GetThumbnail returns one thumbnail scoped to its content item.
func GetThumbnail(ctx context.Context, db *sql.DB, id, contentItemID int64) (Thumbnail, error) {
	t, err := scanThumbnail(db.QueryRowContext(ctx,
		`SELECT id, content_item_id, name, canvas_json, created_at, updated_at
		 FROM thumbnails WHERE id = ? AND content_item_id = ?`, id, contentItemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Thumbnail{}, ErrThumbnailNotFound
	}
	if err != nil {
		return Thumbnail{}, fmt.Errorf("get thumbnail: %w", err)
	}
	return t, nil
}

// UpdateThumbnailCanvas replaces the canvas JSON for a thumbnail.
func UpdateThumbnailCanvas(ctx context.Context, db *sql.DB, id, contentItemID int64, canvasJSON string) error {
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE thumbnails SET canvas_json = ?, updated_at = ? WHERE id = ? AND content_item_id = ?`,
		canvasJSON, ts, id, contentItemID)
	if err != nil {
		return fmt.Errorf("update thumbnail canvas: %w", err)
	}
	return checkAffected(res, ErrThumbnailNotFound)
}

// DeleteThumbnail removes a thumbnail.
func DeleteThumbnail(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM thumbnails WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete thumbnail: %w", err)
	}
	return checkAffected(res, ErrThumbnailNotFound)
}

func scanThumbnail(sc rowScanner) (Thumbnail, error) {
	var (
		t            Thumbnail
		created, upd string
	)
	if err := sc.Scan(&t.ID, &t.ContentItemID, &t.Name, &t.CanvasJSON, &created, &upd); err != nil {
		return Thumbnail{}, err
	}
	t.CreatedAt = parseTS(created)
	t.UpdatedAt = parseTS(upd)
	return t, nil
}
