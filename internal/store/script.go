package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// DefaultWPM is the fallback words-per-minute used for duration estimates.
const DefaultWPM = 150

// Script is the prose/voiceover for a content item, one per item (SPEC §5.1).
type Script struct {
	ContentItemID int64
	Body          string
	WPM           int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// WordCount returns the number of whitespace-separated words in the body.
func (s Script) WordCount() int { return len(strings.Fields(s.Body)) }

// EstimatedSeconds estimates spoken/read duration from the word count and WPM.
func (s Script) EstimatedSeconds() int {
	if s.WPM <= 0 {
		return 0
	}
	return int(math.Round(float64(s.WordCount()) / float64(s.WPM) * 60))
}

// DurationLabel renders the estimate as "Ns" or "Mm SSs".
func (s Script) DurationLabel() string {
	sec := s.EstimatedSeconds()
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	return fmt.Sprintf("%dm %02ds", sec/60, sec%60)
}

// GetScript returns the content item's script. If none exists yet, it returns an
// empty script with the default WPM (not an error), so the editor can render.
func GetScript(ctx context.Context, db *sql.DB, contentItemID int64) (Script, error) {
	var (
		sc           Script
		created, upd string
	)
	err := db.QueryRowContext(ctx,
		`SELECT content_item_id, body, wpm, created_at, updated_at FROM scripts WHERE content_item_id = ?`,
		contentItemID).
		Scan(&sc.ContentItemID, &sc.Body, &sc.WPM, &created, &upd)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Script{ContentItemID: contentItemID, WPM: DefaultWPM}, nil
	case err != nil:
		return Script{}, fmt.Errorf("get script: %w", err)
	}
	sc.CreatedAt = parseTS(created)
	sc.UpdatedAt = parseTS(upd)
	return sc, nil
}

// SaveScript upserts the script for a content item and returns the saved value.
// A non-positive wpm is replaced with DefaultWPM.
func SaveScript(ctx context.Context, db *sql.DB, contentItemID int64, body string, wpm int) (Script, error) {
	if wpm <= 0 {
		wpm = DefaultWPM
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO scripts (content_item_id, body, wpm, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(content_item_id) DO UPDATE SET
			body = excluded.body,
			wpm = excluded.wpm,
			updated_at = excluded.updated_at`,
		contentItemID, body, wpm, now, now)
	if err != nil {
		return Script{}, fmt.Errorf("save script: %w", err)
	}
	return GetScript(ctx, db, contentItemID)
}
