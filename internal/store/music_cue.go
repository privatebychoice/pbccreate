package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Music-cue errors.
var (
	ErrInvalidMusicCue  = errors.New("music cue requires a title")
	ErrMusicCueNotFound = errors.New("music cue not found")
)

// MusicCue is one entry in a content item's music cue sheet (SPEC §5.16).
// ProviderID/MediaAssetID are 0 when not linked; ProviderName/MediaFilename are
// populated by reads for display.
type MusicCue struct {
	ID            int64
	ContentItemID int64
	ProviderID    int64
	ProviderName  string
	MediaAssetID  int64
	MediaFilename string
	Title         string
	Artist        string
	InPoint       string
	OutPoint      string
	License       string
	Notes         string
	CreatedAt     time.Time
}

// AddMusicCue records a cue on a content item (title required).
func AddMusicCue(ctx context.Context, db *sql.DB, cue MusicCue) (MusicCue, error) {
	cue.Title = strings.TrimSpace(cue.Title)
	if cue.Title == "" {
		return MusicCue{}, ErrInvalidMusicCue
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO music_cues (content_item_id, provider_id, media_asset_id, title, artist, in_point, out_point, license, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cue.ContentItemID, nullableID(cue.ProviderID), nullableID(cue.MediaAssetID), cue.Title,
		strings.TrimSpace(cue.Artist), strings.TrimSpace(cue.InPoint), strings.TrimSpace(cue.OutPoint),
		strings.TrimSpace(cue.License), strings.TrimSpace(cue.Notes), ts)
	if err != nil {
		return MusicCue{}, fmt.Errorf("insert music cue: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return MusicCue{}, fmt.Errorf("music cue last insert id: %w", err)
	}
	cue.ID = id
	return cue, nil
}

// ListMusicCues returns a content item's cue sheet, joined with provider name and
// media filename, ordered by id (entry order).
func ListMusicCues(ctx context.Context, db *sql.DB, contentItemID int64) ([]MusicCue, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, c.content_item_id, COALESCE(c.provider_id, 0), COALESCE(ap.name, ''),
			COALESCE(c.media_asset_id, 0), COALESCE(m.filename, ''),
			c.title, c.artist, c.in_point, c.out_point, c.license, c.notes, c.created_at
		FROM music_cues c
		LEFT JOIN asset_providers ap ON ap.id = c.provider_id
		LEFT JOIN media_assets m ON m.id = c.media_asset_id
		WHERE c.content_item_id = ?
		ORDER BY c.id`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query music cues: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []MusicCue
	for rows.Next() {
		cue, err := scanMusicCue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate music cues: %w", err)
	}
	return out, nil
}

// GetMusicCue returns one cue scoped to its content item.
func GetMusicCue(ctx context.Context, db *sql.DB, id, contentItemID int64) (MusicCue, error) {
	cue, err := scanMusicCue(db.QueryRowContext(ctx, `
		SELECT c.id, c.content_item_id, COALESCE(c.provider_id, 0), COALESCE(ap.name, ''),
			COALESCE(c.media_asset_id, 0), COALESCE(m.filename, ''),
			c.title, c.artist, c.in_point, c.out_point, c.license, c.notes, c.created_at
		FROM music_cues c
		LEFT JOIN asset_providers ap ON ap.id = c.provider_id
		LEFT JOIN media_assets m ON m.id = c.media_asset_id
		WHERE c.id = ? AND c.content_item_id = ?`, id, contentItemID))
	if errors.Is(err, sql.ErrNoRows) {
		return MusicCue{}, ErrMusicCueNotFound
	}
	if err != nil {
		return MusicCue{}, fmt.Errorf("get music cue: %w", err)
	}
	return cue, nil
}

// DeleteMusicCue removes a cue scoped to its content item.
func DeleteMusicCue(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM music_cues WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete music cue: %w", err)
	}
	return checkAffected(res, ErrMusicCueNotFound)
}

func scanMusicCue(sc rowScanner) (MusicCue, error) {
	var (
		cue     MusicCue
		created string
	)
	if err := sc.Scan(&cue.ID, &cue.ContentItemID, &cue.ProviderID, &cue.ProviderName,
		&cue.MediaAssetID, &cue.MediaFilename, &cue.Title, &cue.Artist, &cue.InPoint, &cue.OutPoint,
		&cue.License, &cue.Notes, &created); err != nil {
		return MusicCue{}, err
	}
	cue.CreatedAt = parseTS(created)
	return cue, nil
}
