package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Media catalogue errors.
var (
	ErrInvalidMedia       = errors.New("media path is required")
	ErrInvalidMediaKind   = errors.New("invalid media kind")
	ErrInvalidMediaStatus = errors.New("invalid media status")
	ErrMediaNotFound      = errors.New("media asset not found")
)

// Media vocabularies (SPEC §5.7).
var (
	MediaKinds    = []string{"video", "audio", "image", "other"}
	MediaStatuses = []string{"to_shoot", "recorded", "imported", "edited", "used"}
)

// MediaAsset is a catalogued file linked to a content item (and optionally a
// shot). ShotID is 0 when unlinked; SizeBytes/MTime/LastSeenAt are zero when
// unknown (SPEC §5.7).
type MediaAsset struct {
	ID            int64
	ContentItemID int64
	ShotID        int64
	Path          string
	Filename      string
	Kind          string
	SizeBytes     int64
	MTime         time.Time
	Status        string
	Present       bool
	LastSeenAt    time.Time
	Notes         string

	// Technical metadata from ffprobe (0/"" when unknown; SPEC §5.7).
	DurationSeconds int
	Width           int
	Height          int
	Codec           string
	FPS             float64
	Container       string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// MediaMetadata carries the technical properties written by a probe. It mirrors
// media.Metadata but keeps the store layer independent of the media package.
type MediaMetadata struct {
	DurationSeconds int
	Width           int
	Height          int
	Codec           string
	FPS             float64
	Container       string
}

// ListMediaAssets returns a content item's media assets, missing files first,
// then by filename.
func ListMediaAssets(ctx context.Context, db *sql.DB, contentItemID int64) ([]MediaAsset, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, content_item_id, shot_id, path, filename, kind, size_bytes, mtime,
		       status, present, last_seen_at, notes,
		       duration_seconds, width, height, codec, fps, container,
		       created_at, updated_at
		FROM media_assets
		WHERE content_item_id = ?
		ORDER BY present ASC, filename COLLATE NOCASE`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query media assets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []MediaAsset
	for rows.Next() {
		var (
			a                    MediaAsset
			shotID, size         sql.NullInt64
			mtime, lastSeen      sql.NullString
			present              int
			duration, width, hgt sql.NullInt64
			codec, container     sql.NullString
			fps                  sql.NullFloat64
			created, upd         string
		)
		if err := rows.Scan(&a.ID, &a.ContentItemID, &shotID, &a.Path, &a.Filename, &a.Kind,
			&size, &mtime, &a.Status, &present, &lastSeen, &a.Notes,
			&duration, &width, &hgt, &codec, &fps, &container,
			&created, &upd); err != nil {
			return nil, fmt.Errorf("scan media asset: %w", err)
		}
		a.ShotID = shotID.Int64
		a.SizeBytes = size.Int64
		a.Present = present != 0
		if mtime.Valid {
			a.MTime = parseTS(mtime.String)
		}
		if lastSeen.Valid {
			a.LastSeenAt = parseTS(lastSeen.String)
		}
		a.DurationSeconds = int(duration.Int64)
		a.Width = int(width.Int64)
		a.Height = int(hgt.Int64)
		a.Codec = codec.String
		a.FPS = fps.Float64
		a.Container = container.String
		a.CreatedAt = parseTS(created)
		a.UpdatedAt = parseTS(upd)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media assets: %w", err)
	}
	return out, nil
}

// AddMediaAsset inserts a catalogued media asset. Path is required; filename
// defaults to the path's base; kind/status are validated (defaulting to
// other/recorded). ShotID/size/mtime are optional (0/zero = unset).
func AddMediaAsset(ctx context.Context, db *sql.DB, a MediaAsset) (MediaAsset, error) {
	a.Path = strings.TrimSpace(a.Path)
	if a.Path == "" {
		return MediaAsset{}, ErrInvalidMedia
	}
	a.Filename = strings.TrimSpace(a.Filename)
	if a.Filename == "" {
		a.Filename = filepath.Base(a.Path)
	}
	if a.Kind == "" {
		a.Kind = "other"
	} else if !slices.Contains(MediaKinds, a.Kind) {
		return MediaAsset{}, ErrInvalidMediaKind
	}
	if a.Status == "" {
		a.Status = "recorded"
	} else if !slices.Contains(MediaStatuses, a.Status) {
		return MediaAsset{}, ErrInvalidMediaStatus
	}

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO media_assets
			(content_item_id, shot_id, path, filename, kind, size_bytes, mtime, status, present, last_seen_at, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ContentItemID, nullableID(a.ShotID), a.Path, a.Filename, a.Kind,
		nullableInt(a.SizeBytes), nullableTime(a.MTime), a.Status, boolToInt(a.Present),
		nullableTime(a.LastSeenAt), a.Notes, ts, ts)
	if err != nil {
		return MediaAsset{}, fmt.Errorf("insert media asset: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return MediaAsset{}, fmt.Errorf("media last insert id: %w", err)
	}
	a.ID = id
	a.CreatedAt = now
	a.UpdatedAt = now
	return a, nil
}

// UpdateMediaStatus sets an asset's workflow status, scoped to its content item.
func UpdateMediaStatus(ctx context.Context, db *sql.DB, id, contentItemID int64, status string) error {
	if !slices.Contains(MediaStatuses, status) {
		return ErrInvalidMediaStatus
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE media_assets SET status = ?, updated_at = ? WHERE id = ? AND content_item_id = ?`,
		status, ts, id, contentItemID)
	if err != nil {
		return fmt.Errorf("update media status: %w", err)
	}
	return checkAffected(res, ErrMediaNotFound)
}

// SetMediaPresence records the result of re-stat-ing a file. When present, size,
// mtime, and last_seen_at are refreshed; when absent, only the present flag is
// cleared (last known size/mtime are retained).
func SetMediaPresence(ctx context.Context, db *sql.DB, id, contentItemID int64, present bool, size int64, mtime time.Time) error {
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	var (
		res sql.Result
		err error
	)
	if present {
		res, err = db.ExecContext(ctx, `
			UPDATE media_assets
			SET present = 1, size_bytes = ?, mtime = ?, last_seen_at = ?, updated_at = ?
			WHERE id = ? AND content_item_id = ?`,
			size, mtime.UTC().Format(time.RFC3339), now, now, id, contentItemID)
	} else {
		res, err = db.ExecContext(ctx,
			`UPDATE media_assets SET present = 0, updated_at = ? WHERE id = ? AND content_item_id = ?`,
			now, id, contentItemID)
	}
	if err != nil {
		return fmt.Errorf("set media presence: %w", err)
	}
	return checkAffected(res, ErrMediaNotFound)
}

// UpdateMediaMetadata writes technical properties (from a probe) for an asset.
func UpdateMediaMetadata(ctx context.Context, db *sql.DB, id, contentItemID int64, m MediaMetadata) error {
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE media_assets
		SET duration_seconds = ?, width = ?, height = ?, codec = ?, fps = ?, container = ?, updated_at = ?
		WHERE id = ? AND content_item_id = ?`,
		nullableInt(int64(m.DurationSeconds)), nullableInt(int64(m.Width)), nullableInt(int64(m.Height)),
		nullableStr(m.Codec), nullableFloat(m.FPS), nullableStr(m.Container), ts, id, contentItemID)
	if err != nil {
		return fmt.Errorf("update media metadata: %w", err)
	}
	return checkAffected(res, ErrMediaNotFound)
}

// DeleteMediaAsset removes a catalogued asset (does not touch the file on disk).
func DeleteMediaAsset(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM media_assets WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete media asset: %w", err)
	}
	return checkAffected(res, ErrMediaNotFound)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func nullableInt(n int64) any {
	if n <= 0 {
		return nil
	}
	return n
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableFloat(f float64) any {
	if f <= 0 {
		return nil
	}
	return f
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
