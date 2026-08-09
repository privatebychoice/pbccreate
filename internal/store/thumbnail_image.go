package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrThumbnailImageNotFound is returned when no uploaded image matches an id.
var ErrThumbnailImageNotFound = errors.New("thumbnail image not found")

// ThumbnailImage is an uploaded raster image that thumbnail layers reference.
// The file is stored under the data dir; its path is derived from the id.
type ThumbnailImage struct {
	ID            int64
	ContentItemID int64
	Filename      string
	Width         int
	Height        int
	CreatedAt     time.Time
}

// CreateThumbnailImage records an uploaded image and returns it (with its id).
func CreateThumbnailImage(ctx context.Context, db *sql.DB, contentItemID int64, filename string, width, height int) (ThumbnailImage, error) {
	now := time.Now().UTC().Truncate(time.Second)
	res, err := db.ExecContext(ctx,
		`INSERT INTO thumbnail_images (content_item_id, filename, width, height, created_at) VALUES (?, ?, ?, ?, ?)`,
		contentItemID, filename, width, height, now.Format(time.RFC3339))
	if err != nil {
		return ThumbnailImage{}, fmt.Errorf("insert thumbnail image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ThumbnailImage{}, fmt.Errorf("thumbnail image last insert id: %w", err)
	}
	return ThumbnailImage{ID: id, ContentItemID: contentItemID, Filename: filename, Width: width, Height: height, CreatedAt: now}, nil
}

// GetThumbnailImage returns an uploaded image scoped to its content item.
func GetThumbnailImage(ctx context.Context, db *sql.DB, id, contentItemID int64) (ThumbnailImage, error) {
	var (
		ti      ThumbnailImage
		created string
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, content_item_id, filename, width, height, created_at
		 FROM thumbnail_images WHERE id = ? AND content_item_id = ?`, id, contentItemID).
		Scan(&ti.ID, &ti.ContentItemID, &ti.Filename, &ti.Width, &ti.Height, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return ThumbnailImage{}, ErrThumbnailImageNotFound
	}
	if err != nil {
		return ThumbnailImage{}, fmt.Errorf("get thumbnail image: %w", err)
	}
	ti.CreatedAt = parseTS(created)
	return ti, nil
}
