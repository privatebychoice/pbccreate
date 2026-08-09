package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// License-file errors.
var (
	ErrInvalidLicenseFile  = errors.New("license file requires an original filename")
	ErrLicenseFileNotFound = errors.New("license file not found")
)

// LicenseFile is an uploaded legal document/certificate kept on disk for a
// content item (SPEC §5.11). The bytes live under
// <data_dir>/licenses/<content_item_id>/<StoredName>; this row is metadata only.
// ProviderID is 0 when not linked to a registered provider (§5.20);
// ProviderName is populated by list queries for display.
type LicenseFile struct {
	ID               int64
	ContentItemID    int64
	ProviderID       int64
	ProviderName     string
	OriginalFilename string
	StoredName       string
	Description      string
	AppliesTo        string
	SizeBytes        int64
	UploadedAt       time.Time
}

// CreateLicenseFile inserts a license-file metadata row (original filename
// required) and returns it with its new ID. The caller writes the bytes and
// then records the generated stored name via SetLicenseStored.
func CreateLicenseFile(ctx context.Context, db *sql.DB, lf LicenseFile) (LicenseFile, error) {
	lf.OriginalFilename = strings.TrimSpace(lf.OriginalFilename)
	if lf.OriginalFilename == "" {
		return LicenseFile{}, ErrInvalidLicenseFile
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO license_files
			(content_item_id, provider_id, original_filename, stored_name, description, applies_to, size_bytes, uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lf.ContentItemID, nullableID(lf.ProviderID), lf.OriginalFilename, strings.TrimSpace(lf.StoredName),
		strings.TrimSpace(lf.Description), strings.TrimSpace(lf.AppliesTo), lf.SizeBytes, ts)
	if err != nil {
		return LicenseFile{}, fmt.Errorf("insert license file: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return LicenseFile{}, fmt.Errorf("license file last insert id: %w", err)
	}
	lf.ID = id
	lf.UploadedAt = now
	return lf, nil
}

// SetLicenseStored records the on-disk stored name and byte size once the file
// has been written.
func SetLicenseStored(ctx context.Context, db *sql.DB, id, contentItemID int64, storedName string, size int64) error {
	res, err := db.ExecContext(ctx,
		`UPDATE license_files SET stored_name = ?, size_bytes = ? WHERE id = ? AND content_item_id = ?`,
		storedName, size, id, contentItemID)
	if err != nil {
		return fmt.Errorf("set license stored: %w", err)
	}
	return checkAffected(res, ErrLicenseFileNotFound)
}

// ListLicenseFiles returns a content item's license files, newest first, with
// the linked provider name (blank when unlinked).
func ListLicenseFiles(ctx context.Context, db *sql.DB, contentItemID int64) ([]LicenseFile, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT lf.id, lf.content_item_id, lf.provider_id, COALESCE(ap.name, ''),
			lf.original_filename, lf.stored_name, lf.description, lf.applies_to, lf.size_bytes, lf.uploaded_at
		FROM license_files lf
		LEFT JOIN asset_providers ap ON ap.id = lf.provider_id
		WHERE lf.content_item_id = ?
		ORDER BY lf.id DESC`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query license files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []LicenseFile
	for rows.Next() {
		lf, err := scanLicenseFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate license files: %w", err)
	}
	return out, nil
}

// GetLicenseFile returns one license file scoped to its content item, or
// ErrLicenseFileNotFound.
func GetLicenseFile(ctx context.Context, db *sql.DB, id, contentItemID int64) (LicenseFile, error) {
	lf, err := scanLicenseFile(db.QueryRowContext(ctx, `
		SELECT lf.id, lf.content_item_id, lf.provider_id, COALESCE(ap.name, ''),
			lf.original_filename, lf.stored_name, lf.description, lf.applies_to, lf.size_bytes, lf.uploaded_at
		FROM license_files lf
		LEFT JOIN asset_providers ap ON ap.id = lf.provider_id
		WHERE lf.id = ? AND lf.content_item_id = ?`, id, contentItemID))
	if errors.Is(err, sql.ErrNoRows) {
		return LicenseFile{}, ErrLicenseFileNotFound
	}
	if err != nil {
		return LicenseFile{}, fmt.Errorf("get license file: %w", err)
	}
	return lf, nil
}

// DeleteLicenseFile removes a license-file row scoped to its content item.
func DeleteLicenseFile(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM license_files WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete license file: %w", err)
	}
	return checkAffected(res, ErrLicenseFileNotFound)
}

func scanLicenseFile(sc rowScanner) (LicenseFile, error) {
	var (
		lf         LicenseFile
		providerID sql.NullInt64
		uploaded   string
	)
	if err := sc.Scan(&lf.ID, &lf.ContentItemID, &providerID, &lf.ProviderName,
		&lf.OriginalFilename, &lf.StoredName, &lf.Description, &lf.AppliesTo, &lf.SizeBytes, &uploaded); err != nil {
		return LicenseFile{}, err
	}
	if providerID.Valid {
		lf.ProviderID = providerID.Int64
	}
	lf.UploadedAt = parseTS(uploaded)
	return lf, nil
}
