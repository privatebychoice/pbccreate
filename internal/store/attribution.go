package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidAttribution is returned when an attribution has no asset name.
var ErrInvalidAttribution = errors.New("attribution name is required")

// ErrAttributionNotFound is returned when an attribution id does not exist for
// the given content item.
var ErrAttributionNotFound = errors.New("attribution not found")

// AttributionKinds is the fixed vocabulary of asset kinds (SPEC §5.11); enforced
// by a CHECK in the schema and validated on write.
var AttributionKinds = []string{"music", "sfx", "stock", "image", "font", "other"}

// Attribution records a third-party asset a content item used and what crediting
// it requires (SPEC §5.11). MediaAssetID is 0 when not linked to a catalogued
// media asset (§5.7).
type Attribution struct {
	ID                    int64
	ContentItemID         int64
	Name                  string
	Kind                  string
	Provider              string
	License               string
	LicenseID             string
	CreditText            string
	SourceURL             string
	MediaAssetID          int64
	ProviderID            int64  // 0 = not linked to a registered provider (§5.20)
	ProviderName          string // populated by list queries for display
	IncludedInDescription bool
}

// normAttributionKind returns a valid kind, defaulting unknown/empty to "other".
func normAttributionKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	for _, k := range AttributionKinds {
		if k == kind {
			return kind
		}
	}
	return "other"
}

// CreateAttribution adds an attribution to a content item.
func CreateAttribution(ctx context.Context, db *sql.DB, a Attribution) (Attribution, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return Attribution{}, ErrInvalidAttribution
	}
	a.Kind = normAttributionKind(a.Kind)

	included := 0
	if a.IncludedInDescription {
		included = 1
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO attributions
			(content_item_id, name, kind, provider, license, license_id, credit_text, source_url, media_asset_id, provider_id, included_in_description, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ContentItemID, a.Name, a.Kind, strings.TrimSpace(a.Provider), strings.TrimSpace(a.License),
		strings.TrimSpace(a.LicenseID), strings.TrimSpace(a.CreditText), strings.TrimSpace(a.SourceURL),
		nullableID(a.MediaAssetID), nullableID(a.ProviderID), included, ts)
	if err != nil {
		return Attribution{}, fmt.Errorf("insert attribution: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Attribution{}, fmt.Errorf("attribution last insert id: %w", err)
	}
	a.ID = id
	return a, nil
}

// ListAttributions returns a content item's attributions, newest first.
func ListAttributions(ctx context.Context, db *sql.DB, contentItemID int64) ([]Attribution, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.content_item_id, a.name, a.kind, a.provider, a.license, a.license_id,
			a.credit_text, a.source_url, a.media_asset_id, a.provider_id, COALESCE(ap.name, ''), a.included_in_description
		FROM attributions a
		LEFT JOIN asset_providers ap ON ap.id = a.provider_id
		WHERE a.content_item_id = ?
		ORDER BY a.id DESC`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query attributions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Attribution
	for rows.Next() {
		a, err := scanAttribution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attributions: %w", err)
	}
	return out, nil
}

// ToggleAttributionIncluded flips whether an attribution feeds the description
// credits block.
func ToggleAttributionIncluded(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx, `
		UPDATE attributions
		SET included_in_description = 1 - included_in_description
		WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("toggle attribution: %w", err)
	}
	return checkAffected(res, ErrAttributionNotFound)
}

// DeleteAttribution removes an attribution from a content item.
func DeleteAttribution(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM attributions WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete attribution: %w", err)
	}
	return checkAffected(res, ErrAttributionNotFound)
}

// scanAttribution reads one row (nullable media_asset_id maps to 0 when absent).
func scanAttribution(sc rowScanner) (Attribution, error) {
	var (
		a          Attribution
		mediaID    sql.NullInt64
		providerID sql.NullInt64
	)
	if err := sc.Scan(&a.ID, &a.ContentItemID, &a.Name, &a.Kind, &a.Provider, &a.License,
		&a.LicenseID, &a.CreditText, &a.SourceURL, &mediaID, &providerID, &a.ProviderName, &a.IncludedInDescription); err != nil {
		return Attribution{}, fmt.Errorf("scan attribution: %w", err)
	}
	if mediaID.Valid {
		a.MediaAssetID = mediaID.Int64
	}
	if providerID.Valid {
		a.ProviderID = providerID.Int64
	}
	return a, nil
}
