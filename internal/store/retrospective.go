package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Retrospective is the post-publish reflection for a content item, one per item
// (SPEC §5.12). Performance notes are entered by hand — pbccreate makes no
// network calls, so it fetches no analytics.
type Retrospective struct {
	ContentItemID    int64
	WhatWorked       string
	ToImprove        string
	PerformanceNotes string
	ReviewedOn       string // YYYY-MM-DD or ""
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// HasContent reports whether the retrospective carries any operator input (used
// by the readiness view / display).
func (r Retrospective) HasContent() bool {
	return strings.TrimSpace(r.WhatWorked) != "" ||
		strings.TrimSpace(r.ToImprove) != "" ||
		strings.TrimSpace(r.PerformanceNotes) != "" ||
		strings.TrimSpace(r.ReviewedOn) != ""
}

// GetRetrospective returns the item's retrospective, or an empty one (not an
// error) when none exists yet, so the editor can render.
func GetRetrospective(ctx context.Context, db *sql.DB, contentItemID int64) (Retrospective, error) {
	var (
		r            Retrospective
		created, upd string
	)
	err := db.QueryRowContext(ctx,
		`SELECT content_item_id, what_worked, to_improve, performance_notes, reviewed_on, created_at, updated_at
		 FROM retrospectives WHERE content_item_id = ?`, contentItemID).
		Scan(&r.ContentItemID, &r.WhatWorked, &r.ToImprove, &r.PerformanceNotes, &r.ReviewedOn, &created, &upd)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Retrospective{ContentItemID: contentItemID}, nil
	case err != nil:
		return Retrospective{}, fmt.Errorf("get retrospective: %w", err)
	}
	r.CreatedAt = parseTS(created)
	r.UpdatedAt = parseTS(upd)
	return r, nil
}

// SaveRetrospective upserts the retrospective for a content item and returns it.
func SaveRetrospective(ctx context.Context, db *sql.DB, r Retrospective) (Retrospective, error) {
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO retrospectives (content_item_id, what_worked, to_improve, performance_notes, reviewed_on, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(content_item_id) DO UPDATE SET
			what_worked = excluded.what_worked,
			to_improve = excluded.to_improve,
			performance_notes = excluded.performance_notes,
			reviewed_on = excluded.reviewed_on,
			updated_at = excluded.updated_at`,
		r.ContentItemID, strings.TrimSpace(r.WhatWorked), strings.TrimSpace(r.ToImprove),
		strings.TrimSpace(r.PerformanceNotes), strings.TrimSpace(r.ReviewedOn), now, now)
	if err != nil {
		return Retrospective{}, fmt.Errorf("save retrospective: %w", err)
	}
	return GetRetrospective(ctx, db, r.ContentItemID)
}
