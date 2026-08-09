package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTakeNotFound is returned when a take id does not exist for the given shot.
var ErrTakeNotFound = errors.New("take not found")

// Take records one take against a shot-list row (SPEC §5.15). MediaAssetID is 0
// when not linked; MediaFilename is populated by reads for display. Circled marks
// the good/selected take.
type Take struct {
	ID            int64
	ShotID        int64
	MediaAssetID  int64
	MediaFilename string
	Label         string
	Rating        int
	Circled       bool
	Notes         string
	CreatedAt     time.Time
}

// clampRating bounds a rating to 0..5 (0 = unrated).
func clampRating(n int) int {
	switch {
	case n < 0:
		return 0
	case n > 5:
		return 5
	default:
		return n
	}
}

// ShotExists reports whether a shot belongs to the given content item — used to
// scope take operations to the item that owns the shot.
func ShotExists(ctx context.Context, db *sql.DB, shotID, contentItemID int64) (bool, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM shots WHERE id = ? AND content_item_id = ?`, shotID, contentItemID).Scan(&n); err != nil {
		return false, fmt.Errorf("check shot exists: %w", err)
	}
	return n > 0, nil
}

// AddTake records a take against a shot.
func AddTake(ctx context.Context, db *sql.DB, shotID int64, tk Take) error {
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	circled := 0
	if tk.Circled {
		circled = 1
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO takes (shot_id, media_asset_id, label, rating, circled, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		shotID, nullableID(tk.MediaAssetID), strings.TrimSpace(tk.Label), clampRating(tk.Rating), circled, strings.TrimSpace(tk.Notes), ts)
	if err != nil {
		return fmt.Errorf("insert take: %w", err)
	}
	return nil
}

// TakesByShot returns a map of shot id -> its takes for a content item, so the
// detail page can render takes under each shot in one query.
func TakesByShot(ctx context.Context, db *sql.DB, contentItemID int64) (map[int64][]Take, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.shot_id, COALESCE(t.media_asset_id, 0), COALESCE(m.filename, ''),
			t.label, t.rating, t.circled, t.notes, t.created_at
		FROM takes t
		JOIN shots s ON s.id = t.shot_id
		LEFT JOIN media_assets m ON m.id = t.media_asset_id
		WHERE s.content_item_id = ?
		ORDER BY t.shot_id, t.id`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query takes by shot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64][]Take)
	for rows.Next() {
		var (
			tk      Take
			created string
		)
		if err := rows.Scan(&tk.ID, &tk.ShotID, &tk.MediaAssetID, &tk.MediaFilename,
			&tk.Label, &tk.Rating, &tk.Circled, &tk.Notes, &created); err != nil {
			return nil, fmt.Errorf("scan take: %w", err)
		}
		tk.CreatedAt = parseTS(created)
		out[tk.ShotID] = append(out[tk.ShotID], tk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate takes: %w", err)
	}
	return out, nil
}

// ListTakesForShot returns a single shot's takes (used by tests/utilities).
func ListTakesForShot(ctx context.Context, db *sql.DB, shotID int64) ([]Take, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.shot_id, COALESCE(t.media_asset_id, 0), COALESCE(m.filename, ''),
			t.label, t.rating, t.circled, t.notes, t.created_at
		FROM takes t
		LEFT JOIN media_assets m ON m.id = t.media_asset_id
		WHERE t.shot_id = ?
		ORDER BY t.id`, shotID)
	if err != nil {
		return nil, fmt.Errorf("query takes for shot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Take
	for rows.Next() {
		var (
			tk      Take
			created string
		)
		if err := rows.Scan(&tk.ID, &tk.ShotID, &tk.MediaAssetID, &tk.MediaFilename,
			&tk.Label, &tk.Rating, &tk.Circled, &tk.Notes, &created); err != nil {
			return nil, fmt.Errorf("scan take: %w", err)
		}
		tk.CreatedAt = parseTS(created)
		out = append(out, tk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate takes: %w", err)
	}
	return out, nil
}

// ToggleTakeCircled flips a take's circle marker, scoped to its shot.
func ToggleTakeCircled(ctx context.Context, db *sql.DB, takeID, shotID int64) error {
	res, err := db.ExecContext(ctx,
		`UPDATE takes SET circled = 1 - circled WHERE id = ? AND shot_id = ?`, takeID, shotID)
	if err != nil {
		return fmt.Errorf("toggle take circled: %w", err)
	}
	return checkAffected(res, ErrTakeNotFound)
}

// DeleteTake removes a take scoped to its shot.
func DeleteTake(ctx context.Context, db *sql.DB, takeID, shotID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM takes WHERE id = ? AND shot_id = ?`, takeID, shotID)
	if err != nil {
		return fmt.Errorf("delete take: %w", err)
	}
	return checkAffected(res, ErrTakeNotFound)
}
