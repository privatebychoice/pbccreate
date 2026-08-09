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

// Idea errors and vocabulary.
var (
	ErrInvalidIdea  = errors.New("idea title is required")
	ErrIdeaNotFound = errors.New("idea not found")

	// IdeaStatuses is the operator-settable lifecycle. "promoted" is set by the
	// system when an idea becomes a ContentItem, not chosen directly.
	IdeaStatuses = []string{"open", "parked", "dropped", "promoted"}
)

// Idea is a lightweight, channel-scoped pre-ContentItem capture (SPEC §5.13).
// ICE fields are 0 when unset; PromotedContentItemID is 0 until promoted.
type Idea struct {
	ID                    int64
	ChannelID             int64
	ChannelName           string // populated by list/get for display
	Title                 string
	Note                  string
	Source                string
	Impact                int
	Confidence            int
	Effort                int
	Status                string
	PromotedContentItemID int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Score is the computed ICE-style priority: impact*confidence/effort. It is 0
// (unscored) unless all three factors are set, so partially-scored ideas sort
// below fully-scored ones rather than dividing by zero.
func (i Idea) Score() float64 {
	if i.Impact <= 0 || i.Confidence <= 0 || i.Effort <= 0 {
		return 0
	}
	return float64(i.Impact*i.Confidence) / float64(i.Effort)
}

// clampFactor bounds an ICE factor to 0..10 (0 = unset).
func clampFactor(n int) int {
	switch {
	case n < 0:
		return 0
	case n > 10:
		return 10
	default:
		return n
	}
}

// normIdeaStatus keeps a valid status or defaults to "open".
func normIdeaStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if slices.Contains(IdeaStatuses, s) {
		return s
	}
	return "open"
}

const ideaColumns = `i.id, i.channel_id, COALESCE(c.name, ''), i.title, i.note, i.source,
	i.ice_impact, i.ice_confidence, i.ice_effort, i.status, i.promoted_content_item_id,
	i.created_at, i.updated_at`

// CreateIdea inserts an idea (title required) and returns it.
func CreateIdea(ctx context.Context, db *sql.DB, idea Idea) (Idea, error) {
	idea.Title = strings.TrimSpace(idea.Title)
	if idea.Title == "" {
		return Idea{}, ErrInvalidIdea
	}
	idea.Status = normIdeaStatus(idea.Status)
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO ideas (channel_id, title, note, source, ice_impact, ice_confidence, ice_effort, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idea.ChannelID, idea.Title, strings.TrimSpace(idea.Note), strings.TrimSpace(idea.Source),
		clampFactor(idea.Impact), clampFactor(idea.Confidence), clampFactor(idea.Effort), idea.Status, ts, ts)
	if err != nil {
		return Idea{}, fmt.Errorf("insert idea: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Idea{}, fmt.Errorf("idea last insert id: %w", err)
	}
	return GetIdea(ctx, db, id)
}

// ListIdeas returns all ideas with their channel name, highest ICE score first,
// then newest.
func ListIdeas(ctx context.Context, db *sql.DB) ([]Idea, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+ideaColumns+` FROM ideas i LEFT JOIN channels c ON c.id = i.channel_id`)
	if err != nil {
		return nil, fmt.Errorf("query ideas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Idea
	for rows.Next() {
		idea, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, idea)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ideas: %w", err)
	}
	// Sort by computed score (desc), then most recent — done in Go since the
	// score is not stored.
	slices.SortStableFunc(out, func(a, b Idea) int {
		if sa, sb := a.Score(), b.Score(); sa != sb {
			if sa > sb {
				return -1
			}
			return 1
		}
		return int(b.ID - a.ID)
	})
	return out, nil
}

// GetIdea returns one idea with its channel name, or ErrIdeaNotFound.
func GetIdea(ctx context.Context, db *sql.DB, id int64) (Idea, error) {
	idea, err := scanIdea(db.QueryRowContext(ctx,
		`SELECT `+ideaColumns+` FROM ideas i LEFT JOIN channels c ON c.id = i.channel_id WHERE i.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Idea{}, ErrIdeaNotFound
	}
	if err != nil {
		return Idea{}, fmt.Errorf("get idea: %w", err)
	}
	return idea, nil
}

// UpdateIdea updates an idea's editable fields (title required). It does not
// change promotion linkage.
func UpdateIdea(ctx context.Context, db *sql.DB, idea Idea) error {
	idea.Title = strings.TrimSpace(idea.Title)
	if idea.Title == "" {
		return ErrInvalidIdea
	}
	idea.Status = normIdeaStatus(idea.Status)
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE ideas SET
			title = ?, note = ?, source = ?, ice_impact = ?, ice_confidence = ?, ice_effort = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		idea.Title, strings.TrimSpace(idea.Note), strings.TrimSpace(idea.Source),
		clampFactor(idea.Impact), clampFactor(idea.Confidence), clampFactor(idea.Effort), idea.Status, ts, idea.ID)
	if err != nil {
		return fmt.Errorf("update idea: %w", err)
	}
	return checkAffected(res, ErrIdeaNotFound)
}

// MarkIdeaPromoted links an idea to the ContentItem it became and sets its status
// to "promoted".
func MarkIdeaPromoted(ctx context.Context, db *sql.DB, ideaID, contentItemID int64) error {
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE ideas SET status = 'promoted', promoted_content_item_id = ?, updated_at = ? WHERE id = ?`,
		contentItemID, ts, ideaID)
	if err != nil {
		return fmt.Errorf("mark idea promoted: %w", err)
	}
	return checkAffected(res, ErrIdeaNotFound)
}

// DeleteIdea removes an idea.
func DeleteIdea(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM ideas WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete idea: %w", err)
	}
	return checkAffected(res, ErrIdeaNotFound)
}

func scanIdea(sc rowScanner) (Idea, error) {
	var (
		idea         Idea
		promoted     sql.NullInt64
		created, upd string
	)
	if err := sc.Scan(&idea.ID, &idea.ChannelID, &idea.ChannelName, &idea.Title, &idea.Note, &idea.Source,
		&idea.Impact, &idea.Confidence, &idea.Effort, &idea.Status, &promoted, &created, &upd); err != nil {
		return Idea{}, err
	}
	if promoted.Valid {
		idea.PromotedContentItemID = promoted.Int64
	}
	idea.CreatedAt = parseTS(created)
	idea.UpdatedAt = parseTS(upd)
	return idea, nil
}
