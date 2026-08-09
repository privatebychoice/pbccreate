package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Pillar errors.
var (
	ErrInvalidPillar  = errors.New("pillar name is required")
	ErrPillarNotFound = errors.New("pillar not found")
)

// Pillar is a channel content theme (SPEC §5.13). Distinct from project labels
// (§5.14): pillars are strategic themes with a description; labels are terse
// organizational tags.
type Pillar struct {
	ID          int64
	ChannelID   int64
	ChannelName string // populated by list/get for display
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PillarCoverage pairs a pillar with the number of content items assigned to it,
// for the coverage-balance view.
type PillarCoverage struct {
	Pillar
	ItemCount int
}

// CreatePillar inserts a pillar (name required) and returns it.
func CreatePillar(ctx context.Context, db *sql.DB, channelID int64, name, description string) (Pillar, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Pillar{}, ErrInvalidPillar
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO pillars (channel_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		channelID, name, strings.TrimSpace(description), ts, ts)
	if err != nil {
		return Pillar{}, fmt.Errorf("insert pillar: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Pillar{}, fmt.Errorf("pillar last insert id: %w", err)
	}
	return GetPillar(ctx, db, id)
}

// GetOrCreatePillar returns the channel's pillar with the given name
// (case-insensitive), creating it (no description) if absent — used for inline
// assignment on the content item.
func GetOrCreatePillar(ctx context.Context, db *sql.DB, channelID int64, name string) (Pillar, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Pillar{}, ErrInvalidPillar
	}
	var id int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM pillars WHERE channel_id = ? AND name = ? COLLATE NOCASE`, channelID, name).Scan(&id)
	switch {
	case err == nil:
		return GetPillar(ctx, db, id)
	case !errors.Is(err, sql.ErrNoRows):
		return Pillar{}, fmt.Errorf("lookup pillar: %w", err)
	}
	return CreatePillar(ctx, db, channelID, name, "")
}

// GetPillar returns one pillar with its channel name, or ErrPillarNotFound.
func GetPillar(ctx context.Context, db *sql.DB, id int64) (Pillar, error) {
	p, err := scanPillar(db.QueryRowContext(ctx, `
		SELECT p.id, p.channel_id, COALESCE(c.name, ''), p.name, p.description, p.created_at, p.updated_at
		FROM pillars p LEFT JOIN channels c ON c.id = p.channel_id
		WHERE p.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Pillar{}, ErrPillarNotFound
	}
	if err != nil {
		return Pillar{}, fmt.Errorf("get pillar: %w", err)
	}
	return p, nil
}

// ListPillarsForChannel returns a channel's pillars ordered by name.
func ListPillarsForChannel(ctx context.Context, db *sql.DB, channelID int64) ([]Pillar, error) {
	return queryPillars(ctx, db, `
		SELECT p.id, p.channel_id, COALESCE(c.name, ''), p.name, p.description, p.created_at, p.updated_at
		FROM pillars p LEFT JOIN channels c ON c.id = p.channel_id
		WHERE p.channel_id = ? ORDER BY p.name COLLATE NOCASE`, channelID)
}

// ListPillarsForItem returns the pillars assigned to a content item.
func ListPillarsForItem(ctx context.Context, db *sql.DB, contentItemID int64) ([]Pillar, error) {
	return queryPillars(ctx, db, `
		SELECT p.id, p.channel_id, COALESCE(c.name, ''), p.name, p.description, p.created_at, p.updated_at
		FROM content_item_pillars cip
		JOIN pillars p ON p.id = cip.pillar_id
		LEFT JOIN channels c ON c.id = p.channel_id
		WHERE cip.content_item_id = ?
		ORDER BY p.name COLLATE NOCASE`, contentItemID)
}

// ListPillarCoverage returns every pillar with its assigned-item count, ordered
// by channel then name — the coverage-balance view (SPEC §5.13).
func ListPillarCoverage(ctx context.Context, db *sql.DB) ([]PillarCoverage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.id, p.channel_id, COALESCE(c.name, ''), p.name, p.description, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM content_item_pillars cip WHERE cip.pillar_id = p.id)
		FROM pillars p LEFT JOIN channels c ON c.id = p.channel_id
		ORDER BY c.name COLLATE NOCASE, p.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query pillar coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PillarCoverage
	for rows.Next() {
		var (
			pc           PillarCoverage
			created, upd string
		)
		if err := rows.Scan(&pc.ID, &pc.ChannelID, &pc.ChannelName, &pc.Name, &pc.Description,
			&created, &upd, &pc.ItemCount); err != nil {
			return nil, fmt.Errorf("scan pillar coverage: %w", err)
		}
		pc.CreatedAt = parseTS(created)
		pc.UpdatedAt = parseTS(upd)
		out = append(out, pc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pillar coverage: %w", err)
	}
	return out, nil
}

// UpdatePillar updates a pillar's name/description (name required).
func UpdatePillar(ctx context.Context, db *sql.DB, id int64, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidPillar
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE pillars SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		name, strings.TrimSpace(description), ts, id)
	if err != nil {
		return fmt.Errorf("update pillar: %w", err)
	}
	return checkAffected(res, ErrPillarNotFound)
}

// DeletePillar removes a pillar (and its assignments, by cascade).
func DeletePillar(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM pillars WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete pillar: %w", err)
	}
	return checkAffected(res, ErrPillarNotFound)
}

// AssignPillar links a pillar to a content item (idempotent).
func AssignPillar(ctx context.Context, db *sql.DB, contentItemID, pillarID int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO content_item_pillars (content_item_id, pillar_id) VALUES (?, ?)`,
		contentItemID, pillarID)
	if err != nil {
		return fmt.Errorf("assign pillar: %w", err)
	}
	return nil
}

// UnassignPillar removes a pillar from a content item.
func UnassignPillar(ctx context.Context, db *sql.DB, contentItemID, pillarID int64) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM content_item_pillars WHERE content_item_id = ? AND pillar_id = ?`,
		contentItemID, pillarID)
	if err != nil {
		return fmt.Errorf("unassign pillar: %w", err)
	}
	return nil
}

func queryPillars(ctx context.Context, db *sql.DB, query string, args ...any) ([]Pillar, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pillars: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Pillar
	for rows.Next() {
		p, err := scanPillar(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pillars: %w", err)
	}
	return out, nil
}

func scanPillar(sc rowScanner) (Pillar, error) {
	var (
		p            Pillar
		created, upd string
	)
	if err := sc.Scan(&p.ID, &p.ChannelID, &p.ChannelName, &p.Name, &p.Description, &created, &upd); err != nil {
		return Pillar{}, err
	}
	p.CreatedAt = parseTS(created)
	p.UpdatedAt = parseTS(upd)
	return p, nil
}
