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

// Asset-library errors and vocabulary (SPEC §5.16).
var (
	ErrInvalidLibraryAsset  = errors.New("asset name is required")
	ErrLibraryAssetNotFound = errors.New("library asset not found")

	// AssetLibraryKinds is the reusable-asset vocabulary.
	AssetLibraryKinds = []string{"b_roll", "music", "sfx", "graphic", "brand", "other"}
)

// LibraryAsset is one entry in the cross-project asset library. ProviderID is 0
// when not linked; ProviderName is populated by reads for display.
type LibraryAsset struct {
	ID           int64
	Kind         string
	Name         string
	Path         string
	Tags         string
	License      string
	ProviderID   int64
	ProviderName string
	Notes        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func normLibraryKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if slices.Contains(AssetLibraryKinds, kind) {
		return kind
	}
	return "other"
}

// CreateLibraryAsset inserts a library entry (name required).
func CreateLibraryAsset(ctx context.Context, db *sql.DB, a LibraryAsset) (LibraryAsset, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return LibraryAsset{}, ErrInvalidLibraryAsset
	}
	a.Kind = normLibraryKind(a.Kind)
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO asset_library (kind, name, path, tags, license, provider_id, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Kind, a.Name, strings.TrimSpace(a.Path), strings.TrimSpace(a.Tags),
		strings.TrimSpace(a.License), nullableID(a.ProviderID), strings.TrimSpace(a.Notes), ts, ts)
	if err != nil {
		return LibraryAsset{}, fmt.Errorf("insert library asset: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return LibraryAsset{}, fmt.Errorf("library asset last insert id: %w", err)
	}
	return GetLibraryAsset(ctx, db, id)
}

// GetLibraryAsset returns one entry with its provider name.
func GetLibraryAsset(ctx context.Context, db *sql.DB, id int64) (LibraryAsset, error) {
	a, err := scanLibraryAsset(db.QueryRowContext(ctx, `
		SELECT a.id, a.kind, a.name, a.path, a.tags, a.license, COALESCE(a.provider_id, 0), COALESCE(p.name, ''), a.notes, a.created_at, a.updated_at
		FROM asset_library a LEFT JOIN asset_providers p ON p.id = a.provider_id
		WHERE a.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return LibraryAsset{}, ErrLibraryAssetNotFound
	}
	if err != nil {
		return LibraryAsset{}, fmt.Errorf("get library asset: %w", err)
	}
	return a, nil
}

// ListLibraryAssets returns entries filtered by an optional kind and a free-text
// query (matched against name, tags, path, and notes), ordered by name. An empty
// kind/query means no filter on that dimension.
func ListLibraryAssets(ctx context.Context, db *sql.DB, query, kind string) ([]LibraryAsset, error) {
	var (
		where []string
		args  []any
	)
	if k := strings.ToLower(strings.TrimSpace(kind)); k != "" && slices.Contains(AssetLibraryKinds, k) {
		where = append(where, "a.kind = ?")
		args = append(args, k)
	}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		where = append(where, "(a.name LIKE ? OR a.tags LIKE ? OR a.path LIKE ? OR a.notes LIKE ?)")
		args = append(args, like, like, like, like)
	}
	sqlStr := `
		SELECT a.id, a.kind, a.name, a.path, a.tags, a.license, COALESCE(a.provider_id, 0), COALESCE(p.name, ''), a.notes, a.created_at, a.updated_at
		FROM asset_library a LEFT JOIN asset_providers p ON p.id = a.provider_id`
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY a.name COLLATE NOCASE"

	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query library assets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []LibraryAsset
	for rows.Next() {
		a, err := scanLibraryAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate library assets: %w", err)
	}
	return out, nil
}

// UpdateLibraryAsset updates an entry (name required).
func UpdateLibraryAsset(ctx context.Context, db *sql.DB, a LibraryAsset) error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return ErrInvalidLibraryAsset
	}
	a.Kind = normLibraryKind(a.Kind)
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE asset_library SET kind = ?, name = ?, path = ?, tags = ?, license = ?, provider_id = ?, notes = ?, updated_at = ?
		WHERE id = ?`,
		a.Kind, a.Name, strings.TrimSpace(a.Path), strings.TrimSpace(a.Tags),
		strings.TrimSpace(a.License), nullableID(a.ProviderID), strings.TrimSpace(a.Notes), ts, a.ID)
	if err != nil {
		return fmt.Errorf("update library asset: %w", err)
	}
	return checkAffected(res, ErrLibraryAssetNotFound)
}

// DeleteLibraryAsset removes an entry.
func DeleteLibraryAsset(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM asset_library WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete library asset: %w", err)
	}
	return checkAffected(res, ErrLibraryAssetNotFound)
}

func scanLibraryAsset(sc rowScanner) (LibraryAsset, error) {
	var (
		a            LibraryAsset
		created, upd string
	)
	if err := sc.Scan(&a.ID, &a.Kind, &a.Name, &a.Path, &a.Tags, &a.License,
		&a.ProviderID, &a.ProviderName, &a.Notes, &created, &upd); err != nil {
		return LibraryAsset{}, err
	}
	a.CreatedAt = parseTS(created)
	a.UpdatedAt = parseTS(upd)
	return a, nil
}
