package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Title errors.
var (
	ErrInvalidTitleCandidate  = errors.New("title text is required")
	ErrTitleCandidateNotFound = errors.New("title candidate not found")
	ErrInvalidSwipe           = errors.New("swipe pattern is required")
	ErrSwipeNotFound          = errors.New("swipe entry not found")
)

// TitleCandidate is one A/B title option for a content item (SPEC §5.13).
type TitleCandidate struct {
	ID            int64
	ContentItemID int64
	Text          string
	Chosen        bool
}

// SwipeEntry is one channel title-swipe pattern that worked (SPEC §5.13).
type SwipeEntry struct {
	ID        int64
	ChannelID int64
	Pattern   string
	Note      string
}

// --- title candidates ---

// AddTitleCandidate records a candidate title (text required).
func AddTitleCandidate(ctx context.Context, db *sql.DB, contentItemID int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrInvalidTitleCandidate
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO title_candidates (content_item_id, text, created_at) VALUES (?, ?, ?)`,
		contentItemID, text, ts); err != nil {
		return fmt.Errorf("insert title candidate: %w", err)
	}
	return nil
}

// ListTitleCandidates returns an item's candidates (chosen first, then entry order).
func ListTitleCandidates(ctx context.Context, db *sql.DB, contentItemID int64) ([]TitleCandidate, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, content_item_id, text, chosen FROM title_candidates WHERE content_item_id = ? ORDER BY chosen DESC, id`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query title candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TitleCandidate
	for rows.Next() {
		var c TitleCandidate
		if err := rows.Scan(&c.ID, &c.ContentItemID, &c.Text, &c.Chosen); err != nil {
			return nil, fmt.Errorf("scan title candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate title candidates: %w", err)
	}
	return out, nil
}

// ChooseTitleCandidate marks one candidate as chosen and clears the rest for the
// item, in a single transaction (exactly one chosen at a time).
func ChooseTitleCandidate(ctx context.Context, db *sql.DB, candidateID, contentItemID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin choose tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE title_candidates SET chosen = 1 WHERE id = ? AND content_item_id = ?`, candidateID, contentItemID)
	if err != nil {
		return fmt.Errorf("mark chosen: %w", err)
	}
	if err := checkAffected(res, ErrTitleCandidateNotFound); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE title_candidates SET chosen = 0 WHERE content_item_id = ? AND id <> ?`, contentItemID, candidateID); err != nil {
		return fmt.Errorf("clear other chosen: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit choose: %w", err)
	}
	return nil
}

// DeleteTitleCandidate removes a candidate scoped to its item.
func DeleteTitleCandidate(ctx context.Context, db *sql.DB, candidateID, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM title_candidates WHERE id = ? AND content_item_id = ?`, candidateID, contentItemID)
	if err != nil {
		return fmt.Errorf("delete title candidate: %w", err)
	}
	return checkAffected(res, ErrTitleCandidateNotFound)
}

// --- title swipe file ---

// AddSwipe records a channel title-swipe pattern (pattern required).
func AddSwipe(ctx context.Context, db *sql.DB, channelID int64, pattern, note string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ErrInvalidSwipe
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO title_swipe (channel_id, pattern, note, created_at) VALUES (?, ?, ?, ?)`,
		channelID, pattern, strings.TrimSpace(note), ts); err != nil {
		return fmt.Errorf("insert swipe: %w", err)
	}
	return nil
}

// ListSwipe returns a channel's swipe file, newest first.
func ListSwipe(ctx context.Context, db *sql.DB, channelID int64) ([]SwipeEntry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, channel_id, pattern, note FROM title_swipe WHERE channel_id = ? ORDER BY id DESC`, channelID)
	if err != nil {
		return nil, fmt.Errorf("query swipe: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SwipeEntry
	for rows.Next() {
		var e SwipeEntry
		if err := rows.Scan(&e.ID, &e.ChannelID, &e.Pattern, &e.Note); err != nil {
			return nil, fmt.Errorf("scan swipe: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate swipe: %w", err)
	}
	return out, nil
}

// DeleteSwipe removes a swipe entry scoped to its channel.
func DeleteSwipe(ctx context.Context, db *sql.DB, swipeID, channelID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM title_swipe WHERE id = ? AND channel_id = ?`, swipeID, channelID)
	if err != nil {
		return fmt.Errorf("delete swipe: %w", err)
	}
	return checkAffected(res, ErrSwipeNotFound)
}
