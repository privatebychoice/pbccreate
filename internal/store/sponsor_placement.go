package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Placement / deliverable errors.
var (
	ErrPlacementNotFound   = errors.New("placement not found")
	ErrPlacementExists     = errors.New("campaign already attached to this item")
	ErrInvalidDeliverable  = errors.New("deliverable description is required")
	ErrDeliverableNotFound = errors.New("deliverable not found")
)

// Placement links a sponsor campaign to a content item (SPEC §5.6). SponsorName
// and CampaignName are populated by list/get joins.
type Placement struct {
	ID            int64
	CampaignID    int64
	ContentItemID int64
	Deadline      string
	SponsorName   string
	CampaignName  string
	// Campaign fields (populated by ListPlacementsForItem) used for the
	// description sponsor blurb.
	TalkingPoints string
	PromoCode     string
	TrackingLink  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Deliverable is one checklist item under a placement.
type Deliverable struct {
	ID          int64
	PlacementID int64
	Position    int
	Description string
	Done        bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CampaignOption is a selectable campaign (for attaching a placement).
type CampaignOption struct {
	ID    int64
	Label string // "Sponsor — Campaign"
}

// ListCampaignOptions returns every campaign labeled by sponsor and campaign name.
func ListCampaignOptions(ctx context.Context, db *sql.DB) ([]CampaignOption, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, s.name, c.name
		FROM sponsor_campaigns c
		JOIN sponsors s ON s.id = c.sponsor_id
		ORDER BY s.name COLLATE NOCASE, c.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query campaign options: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CampaignOption
	for rows.Next() {
		var (
			id                    int64
			sponsorName, campaign string
		)
		if err := rows.Scan(&id, &sponsorName, &campaign); err != nil {
			return nil, fmt.Errorf("scan campaign option: %w", err)
		}
		out = append(out, CampaignOption{ID: id, Label: sponsorName + " — " + campaign})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaign options: %w", err)
	}
	return out, nil
}

// CreatePlacement attaches a campaign to a content item.
func CreatePlacement(ctx context.Context, db *sql.DB, campaignID, contentItemID int64, deadline string) (Placement, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sponsor_placements WHERE content_item_id = ? AND campaign_id = ?`,
		contentItemID, campaignID).Scan(&n); err != nil {
		return Placement{}, fmt.Errorf("check existing placement: %w", err)
	}
	if n > 0 {
		return Placement{}, ErrPlacementExists
	}

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO sponsor_placements (campaign_id, content_item_id, deadline, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		campaignID, contentItemID, strings.TrimSpace(deadline), ts, ts)
	if err != nil {
		return Placement{}, fmt.Errorf("insert placement: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Placement{}, fmt.Errorf("placement last insert id: %w", err)
	}
	return Placement{ID: id, CampaignID: campaignID, ContentItemID: contentItemID, Deadline: strings.TrimSpace(deadline), CreatedAt: now, UpdatedAt: now}, nil
}

// ListPlacementsForItem returns a content item's placements with sponsor/campaign
// names, oldest first.
func ListPlacementsForItem(ctx context.Context, db *sql.DB, contentItemID int64) ([]Placement, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.id, p.campaign_id, p.content_item_id, p.deadline, s.name, c.name,
		       c.talking_points, c.promo_code, c.tracking_link, p.created_at, p.updated_at
		FROM sponsor_placements p
		JOIN sponsor_campaigns c ON c.id = p.campaign_id
		JOIN sponsors s ON s.id = c.sponsor_id
		WHERE p.content_item_id = ?
		ORDER BY p.created_at, p.id`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query placements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Placement
	for rows.Next() {
		var (
			p            Placement
			created, upd string
		)
		if err := rows.Scan(&p.ID, &p.CampaignID, &p.ContentItemID, &p.Deadline, &p.SponsorName, &p.CampaignName,
			&p.TalkingPoints, &p.PromoCode, &p.TrackingLink, &created, &upd); err != nil {
			return nil, fmt.Errorf("scan placement: %w", err)
		}
		p.CreatedAt = parseTS(created)
		p.UpdatedAt = parseTS(upd)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate placements: %w", err)
	}
	return out, nil
}

// ContentItemIDsWithPlacements returns the set of content item ids that have at
// least one sponsor placement (for the board "sponsored" badge).
func ContentItemIDsWithPlacements(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT content_item_id FROM sponsor_placements`)
	if err != nil {
		return nil, fmt.Errorf("query placement item ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	set := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan placement item id: %w", err)
		}
		set[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate placement item ids: %w", err)
	}
	return set, nil
}

// GetPlacement returns one placement scoped to its content item.
func GetPlacement(ctx context.Context, db *sql.DB, id, contentItemID int64) (Placement, error) {
	var (
		p            Placement
		created, upd string
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, campaign_id, content_item_id, deadline, created_at, updated_at
		 FROM sponsor_placements WHERE id = ? AND content_item_id = ?`, id, contentItemID).
		Scan(&p.ID, &p.CampaignID, &p.ContentItemID, &p.Deadline, &created, &upd)
	if errors.Is(err, sql.ErrNoRows) {
		return Placement{}, ErrPlacementNotFound
	}
	if err != nil {
		return Placement{}, fmt.Errorf("get placement: %w", err)
	}
	p.CreatedAt = parseTS(created)
	p.UpdatedAt = parseTS(upd)
	return p, nil
}

// DeletePlacement detaches a campaign from a content item.
func DeletePlacement(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM sponsor_placements WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete placement: %w", err)
	}
	return checkAffected(res, ErrPlacementNotFound)
}

// AddDeliverable appends a checklist item to a placement.
func AddDeliverable(ctx context.Context, db *sql.DB, placementID int64, description string) (Deliverable, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return Deliverable{}, ErrInvalidDeliverable
	}
	var pos int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM sponsor_deliverables WHERE placement_id = ?`,
		placementID).Scan(&pos); err != nil {
		return Deliverable{}, fmt.Errorf("next deliverable position: %w", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO sponsor_deliverables (placement_id, position, description, done, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?)`,
		placementID, pos, description, ts, ts)
	if err != nil {
		return Deliverable{}, fmt.Errorf("insert deliverable: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Deliverable{}, fmt.Errorf("deliverable last insert id: %w", err)
	}
	return Deliverable{ID: id, PlacementID: placementID, Position: pos, Description: description, CreatedAt: now, UpdatedAt: now}, nil
}

// ListDeliverables returns a placement's checklist in position order.
func ListDeliverables(ctx context.Context, db *sql.DB, placementID int64) ([]Deliverable, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, placement_id, position, description, done, created_at, updated_at
		 FROM sponsor_deliverables WHERE placement_id = ? ORDER BY position`, placementID)
	if err != nil {
		return nil, fmt.Errorf("query deliverables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Deliverable
	for rows.Next() {
		var (
			d            Deliverable
			done         int
			created, upd string
		)
		if err := rows.Scan(&d.ID, &d.PlacementID, &d.Position, &d.Description, &done, &created, &upd); err != nil {
			return nil, fmt.Errorf("scan deliverable: %w", err)
		}
		d.Done = done != 0
		d.CreatedAt = parseTS(created)
		d.UpdatedAt = parseTS(upd)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deliverables: %w", err)
	}
	return out, nil
}

// ToggleDeliverable flips a deliverable's done state, scoped to its placement.
func ToggleDeliverable(ctx context.Context, db *sql.DB, id, placementID int64) error {
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE sponsor_deliverables SET done = 1 - done, updated_at = ? WHERE id = ? AND placement_id = ?`,
		ts, id, placementID)
	if err != nil {
		return fmt.Errorf("toggle deliverable: %w", err)
	}
	return checkAffected(res, ErrDeliverableNotFound)
}

// DeleteDeliverable removes a checklist item scoped to its placement.
func DeleteDeliverable(ctx context.Context, db *sql.DB, id, placementID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM sponsor_deliverables WHERE id = ? AND placement_id = ?`, id, placementID)
	if err != nil {
		return fmt.Errorf("delete deliverable: %w", err)
	}
	return checkAffected(res, ErrDeliverableNotFound)
}
