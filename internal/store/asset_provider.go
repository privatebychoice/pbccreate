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

// Asset-provider errors and vocabularies (SPEC §5.20).
var (
	ErrInvalidProvider  = errors.New("provider name is required")
	ErrProviderNotFound = errors.New("provider not found")

	// ServiceTypes is the fixed vocabulary of media-service kinds; enforced by a
	// CHECK in the schema and normalized on write (default "other").
	ServiceTypes = []string{"music", "sfx", "stock", "images", "fonts", "other"}
	// ProviderStatuses is the subscription lifecycle vocabulary (default "active").
	ProviderStatuses = []string{"active", "lapsed"}
)

// AssetProvider is a 3rd-party media service the operator subscribes to (SPEC
// §5.20). Operator-wide (not channel-scoped). PortalURL is an account/portal
// link only — never a credential.
type AssetProvider struct {
	ID           int64
	Name         string
	ServiceType  string
	WebsiteURL   string
	PlanTier     string
	BillingCycle string
	RenewalOn    string // YYYY-MM-DD or ""
	Status       string
	TermsNotes   string
	PortalURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// normServiceType keeps a valid service type or defaults to "other".
func normServiceType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if slices.Contains(ServiceTypes, s) {
		return s
	}
	return "other"
}

// normProviderStatus keeps a valid status or defaults to "active".
func normProviderStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if slices.Contains(ProviderStatuses, s) {
		return s
	}
	return "active"
}

const providerColumns = `id, name, service_type, website_url, plan_tier,
	billing_cycle, renewal_on, status, terms_notes, portal_url,
	created_at, updated_at`

// clean trims and normalizes a provider's fields in place (name is validated by
// the caller).
func (p *AssetProvider) clean() {
	p.Name = strings.TrimSpace(p.Name)
	p.ServiceType = normServiceType(p.ServiceType)
	p.WebsiteURL = strings.TrimSpace(p.WebsiteURL)
	p.PlanTier = strings.TrimSpace(p.PlanTier)
	p.BillingCycle = strings.TrimSpace(p.BillingCycle)
	p.RenewalOn = strings.TrimSpace(p.RenewalOn)
	p.Status = normProviderStatus(p.Status)
	p.TermsNotes = strings.TrimSpace(p.TermsNotes)
	p.PortalURL = strings.TrimSpace(p.PortalURL)
}

// CreateAssetProvider inserts a provider (name required) and returns it.
func CreateAssetProvider(ctx context.Context, db *sql.DB, p AssetProvider) (AssetProvider, error) {
	p.clean()
	if p.Name == "" {
		return AssetProvider{}, ErrInvalidProvider
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO asset_providers
			(name, service_type, website_url, plan_tier, billing_cycle, renewal_on, status, terms_notes, portal_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.ServiceType, p.WebsiteURL, p.PlanTier, p.BillingCycle, p.RenewalOn, p.Status, p.TermsNotes, p.PortalURL, ts, ts)
	if err != nil {
		return AssetProvider{}, fmt.Errorf("insert asset provider: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return AssetProvider{}, fmt.Errorf("asset provider last insert id: %w", err)
	}
	p.ID = id
	p.CreatedAt, p.UpdatedAt = now, now
	return p, nil
}

// ListAssetProviders returns all providers ordered case-insensitively by name.
func ListAssetProviders(ctx context.Context, db *sql.DB) ([]AssetProvider, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+providerColumns+` FROM asset_providers ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query asset providers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AssetProvider
	for rows.Next() {
		p, err := scanAssetProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate asset providers: %w", err)
	}
	return out, nil
}

// GetAssetProvider returns one provider, or ErrProviderNotFound.
func GetAssetProvider(ctx context.Context, db *sql.DB, id int64) (AssetProvider, error) {
	p, err := scanAssetProvider(db.QueryRowContext(ctx,
		`SELECT `+providerColumns+` FROM asset_providers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return AssetProvider{}, ErrProviderNotFound
	}
	if err != nil {
		return AssetProvider{}, fmt.Errorf("get asset provider: %w", err)
	}
	return p, nil
}

// UpdateAssetProvider updates a provider's fields (name required).
func UpdateAssetProvider(ctx context.Context, db *sql.DB, p AssetProvider) error {
	p.clean()
	if p.Name == "" {
		return ErrInvalidProvider
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE asset_providers SET
			name = ?, service_type = ?, website_url = ?, plan_tier = ?, billing_cycle = ?,
			renewal_on = ?, status = ?, terms_notes = ?, portal_url = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.ServiceType, p.WebsiteURL, p.PlanTier, p.BillingCycle, p.RenewalOn, p.Status, p.TermsNotes, p.PortalURL, ts, p.ID)
	if err != nil {
		return fmt.Errorf("update asset provider: %w", err)
	}
	return checkAffected(res, ErrProviderNotFound)
}

// DeleteAssetProvider removes a provider. The provider links added later
// (attributions.provider_id, license_files.provider_id) will use ON DELETE SET
// NULL, so deleting a provider clears those references rather than cascading.
func DeleteAssetProvider(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM asset_providers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete asset provider: %w", err)
	}
	return checkAffected(res, ErrProviderNotFound)
}

func scanAssetProvider(sc rowScanner) (AssetProvider, error) {
	var (
		p            AssetProvider
		created, upd string
	)
	if err := sc.Scan(&p.ID, &p.Name, &p.ServiceType, &p.WebsiteURL, &p.PlanTier,
		&p.BillingCycle, &p.RenewalOn, &p.Status, &p.TermsNotes, &p.PortalURL, &created, &upd); err != nil {
		return AssetProvider{}, err
	}
	p.CreatedAt = parseTS(created)
	p.UpdatedAt = parseTS(upd)
	return p, nil
}
