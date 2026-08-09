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

// Campaign errors and vocabularies.
var (
	ErrInvalidCampaign  = errors.New("campaign name is required")
	ErrCampaignNotFound = errors.New("campaign not found")

	InvoiceStatuses = []string{"draft", "sent"}
	PaymentStatuses = []string{"unpaid", "partial", "paid"}
)

// Campaign is a sponsorship campaign under a sponsor (SPEC §5.6). Financial
// fields are optional; RateSet reports whether Rate is present.
type Campaign struct {
	ID            int64
	SponsorID     int64
	Name          string
	StartsOn      string // YYYY-MM-DD or ""
	EndsOn        string
	TalkingPoints string
	PromoCode     string
	TrackingLink  string
	Rate          float64
	RateSet       bool
	Currency      string
	InvoiceStatus string
	PaymentStatus string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// normStatus keeps s only if it is empty or in the allowed set; anything else
// becomes "" (unset).
func normStatus(s string, allowed []string) string {
	s = strings.TrimSpace(s)
	if s == "" || slices.Contains(allowed, s) {
		return s
	}
	return ""
}

const campaignColumns = `id, sponsor_id, name, starts_on, ends_on, talking_points,
	promo_code, tracking_link, rate, currency, invoice_status, payment_status,
	created_at, updated_at`

// CreateCampaign inserts a campaign (name required) and returns it.
func CreateCampaign(ctx context.Context, db *sql.DB, c Campaign) (Campaign, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return Campaign{}, ErrInvalidCampaign
	}
	c.InvoiceStatus = normStatus(c.InvoiceStatus, InvoiceStatuses)
	c.PaymentStatus = normStatus(c.PaymentStatus, PaymentStatuses)

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO sponsor_campaigns
			(sponsor_id, name, starts_on, ends_on, talking_points, promo_code, tracking_link, rate, currency, invoice_status, payment_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.SponsorID, c.Name, c.StartsOn, c.EndsOn, c.TalkingPoints, c.PromoCode, c.TrackingLink,
		rateArg(c), c.Currency, c.InvoiceStatus, c.PaymentStatus, ts, ts)
	if err != nil {
		return Campaign{}, fmt.Errorf("insert campaign: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Campaign{}, fmt.Errorf("campaign last insert id: %w", err)
	}
	c.ID = id
	c.CreatedAt = now
	c.UpdatedAt = now
	return c, nil
}

// ListCampaigns returns a sponsor's campaigns, newest first.
func ListCampaigns(ctx context.Context, db *sql.DB, sponsorID int64) ([]Campaign, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+campaignColumns+` FROM sponsor_campaigns WHERE sponsor_id = ? ORDER BY created_at DESC, id DESC`,
		sponsorID)
	if err != nil {
		return nil, fmt.Errorf("query campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaigns: %w", err)
	}
	return out, nil
}

// GetCampaign returns one campaign scoped to its sponsor, or ErrCampaignNotFound.
func GetCampaign(ctx context.Context, db *sql.DB, id, sponsorID int64) (Campaign, error) {
	c, err := scanCampaign(db.QueryRowContext(ctx,
		`SELECT `+campaignColumns+` FROM sponsor_campaigns WHERE id = ? AND sponsor_id = ?`, id, sponsorID))
	if errors.Is(err, sql.ErrNoRows) {
		return Campaign{}, ErrCampaignNotFound
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("get campaign: %w", err)
	}
	return c, nil
}

// UpdateCampaign updates a campaign scoped to its sponsor (name required).
func UpdateCampaign(ctx context.Context, db *sql.DB, c Campaign) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return ErrInvalidCampaign
	}
	c.InvoiceStatus = normStatus(c.InvoiceStatus, InvoiceStatuses)
	c.PaymentStatus = normStatus(c.PaymentStatus, PaymentStatuses)

	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE sponsor_campaigns SET
			name = ?, starts_on = ?, ends_on = ?, talking_points = ?, promo_code = ?, tracking_link = ?,
			rate = ?, currency = ?, invoice_status = ?, payment_status = ?, updated_at = ?
		WHERE id = ? AND sponsor_id = ?`,
		c.Name, c.StartsOn, c.EndsOn, c.TalkingPoints, c.PromoCode, c.TrackingLink,
		rateArg(c), c.Currency, c.InvoiceStatus, c.PaymentStatus, ts, c.ID, c.SponsorID)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	return checkAffected(res, ErrCampaignNotFound)
}

// DeleteCampaign removes a campaign scoped to its sponsor.
func DeleteCampaign(ctx context.Context, db *sql.DB, id, sponsorID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM sponsor_campaigns WHERE id = ? AND sponsor_id = ?`, id, sponsorID)
	if err != nil {
		return fmt.Errorf("delete campaign: %w", err)
	}
	return checkAffected(res, ErrCampaignNotFound)
}

// rateArg returns the nullable rate value for SQL (nil when unset).
func rateArg(c Campaign) any {
	if c.RateSet {
		return c.Rate
	}
	return nil
}

func scanCampaign(sc rowScanner) (Campaign, error) {
	var (
		c            Campaign
		rate         sql.NullFloat64
		created, upd string
	)
	if err := sc.Scan(&c.ID, &c.SponsorID, &c.Name, &c.StartsOn, &c.EndsOn, &c.TalkingPoints,
		&c.PromoCode, &c.TrackingLink, &rate, &c.Currency, &c.InvoiceStatus, &c.PaymentStatus,
		&created, &upd); err != nil {
		return Campaign{}, err
	}
	c.Rate = rate.Float64
	c.RateSet = rate.Valid
	c.CreatedAt = parseTS(created)
	c.UpdatedAt = parseTS(upd)
	return c, nil
}
