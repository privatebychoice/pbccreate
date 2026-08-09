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

// Shoot-profile errors and vocabulary (SPEC §5.15).
var (
	ErrInvalidProfile  = errors.New("profile name is required")
	ErrProfileNotFound = errors.New("profile not found")

	// ProfileKinds distinguishes gear setups from location/set profiles.
	ProfileKinds = []string{"gear", "location"}
)

// ShootProfile is a reusable channel-scoped gear or location profile.
type ShootProfile struct {
	ID          int64
	ChannelID   int64
	ChannelName string
	Kind        string
	Name        string
	Details     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func normProfileKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if slices.Contains(ProfileKinds, kind) {
		return kind
	}
	return "gear"
}

// CreateShootProfile inserts a profile (name required, kind validated).
func CreateShootProfile(ctx context.Context, db *sql.DB, channelID int64, kind, name, details string) (ShootProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ShootProfile{}, ErrInvalidProfile
	}
	kind = normProfileKind(kind)
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO shoot_profiles (channel_id, kind, name, details, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		channelID, kind, name, strings.TrimSpace(details), ts, ts)
	if err != nil {
		return ShootProfile{}, fmt.Errorf("insert shoot profile: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ShootProfile{}, fmt.Errorf("shoot profile last insert id: %w", err)
	}
	return GetShootProfile(ctx, db, id)
}

// GetOrCreateShootProfile returns the channel's profile of that kind with the
// given name (case-insensitive), creating it (no details) if absent — used for
// inline assignment on the content item.
func GetOrCreateShootProfile(ctx context.Context, db *sql.DB, channelID int64, kind, name string) (ShootProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ShootProfile{}, ErrInvalidProfile
	}
	kind = normProfileKind(kind)
	var id int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM shoot_profiles WHERE channel_id = ? AND kind = ? AND name = ? COLLATE NOCASE`,
		channelID, kind, name).Scan(&id)
	switch {
	case err == nil:
		return GetShootProfile(ctx, db, id)
	case !errors.Is(err, sql.ErrNoRows):
		return ShootProfile{}, fmt.Errorf("lookup shoot profile: %w", err)
	}
	return CreateShootProfile(ctx, db, channelID, kind, name, "")
}

// GetShootProfile returns one profile with its channel name.
func GetShootProfile(ctx context.Context, db *sql.DB, id int64) (ShootProfile, error) {
	p, err := scanShootProfile(db.QueryRowContext(ctx, `
		SELECT p.id, p.channel_id, COALESCE(c.name, ''), p.kind, p.name, p.details, p.created_at, p.updated_at
		FROM shoot_profiles p LEFT JOIN channels c ON c.id = p.channel_id
		WHERE p.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ShootProfile{}, ErrProfileNotFound
	}
	if err != nil {
		return ShootProfile{}, fmt.Errorf("get shoot profile: %w", err)
	}
	return p, nil
}

// ListShootProfiles returns all profiles of a kind (with channel name), ordered
// by channel then name — for the management page.
func ListShootProfiles(ctx context.Context, db *sql.DB, kind string) ([]ShootProfile, error) {
	return queryShootProfiles(ctx, db, `
		SELECT p.id, p.channel_id, COALESCE(c.name, ''), p.kind, p.name, p.details, p.created_at, p.updated_at
		FROM shoot_profiles p LEFT JOIN channels c ON c.id = p.channel_id
		WHERE p.kind = ?
		ORDER BY c.name COLLATE NOCASE, p.name COLLATE NOCASE`, normProfileKind(kind))
}

// ListProfilesForChannel returns a channel's profiles of a kind (for datalists).
func ListProfilesForChannel(ctx context.Context, db *sql.DB, channelID int64, kind string) ([]ShootProfile, error) {
	return queryShootProfiles(ctx, db, `
		SELECT p.id, p.channel_id, '', p.kind, p.name, p.details, p.created_at, p.updated_at
		FROM shoot_profiles p
		WHERE p.channel_id = ? AND p.kind = ?
		ORDER BY p.name COLLATE NOCASE`, channelID, normProfileKind(kind))
}

// ListProfilesForItem returns the profiles of a kind assigned to a content item.
func ListProfilesForItem(ctx context.Context, db *sql.DB, contentItemID int64, kind string) ([]ShootProfile, error) {
	return queryShootProfiles(ctx, db, `
		SELECT p.id, p.channel_id, '', p.kind, p.name, p.details, p.created_at, p.updated_at
		FROM content_item_profiles cip
		JOIN shoot_profiles p ON p.id = cip.profile_id
		WHERE cip.content_item_id = ? AND p.kind = ?
		ORDER BY p.name COLLATE NOCASE`, contentItemID, normProfileKind(kind))
}

// UpdateShootProfile updates a profile's name/details (name required).
func UpdateShootProfile(ctx context.Context, db *sql.DB, id int64, name, details string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidProfile
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE shoot_profiles SET name = ?, details = ?, updated_at = ? WHERE id = ?`,
		name, strings.TrimSpace(details), ts, id)
	if err != nil {
		return fmt.Errorf("update shoot profile: %w", err)
	}
	return checkAffected(res, ErrProfileNotFound)
}

// DeleteShootProfile removes a profile (and its assignments, by cascade).
func DeleteShootProfile(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM shoot_profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete shoot profile: %w", err)
	}
	return checkAffected(res, ErrProfileNotFound)
}

// AssignProfile links a profile to a content item (idempotent).
func AssignProfile(ctx context.Context, db *sql.DB, contentItemID, profileID int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO content_item_profiles (content_item_id, profile_id) VALUES (?, ?)`,
		contentItemID, profileID)
	if err != nil {
		return fmt.Errorf("assign profile: %w", err)
	}
	return nil
}

// UnassignProfile removes a profile from a content item.
func UnassignProfile(ctx context.Context, db *sql.DB, contentItemID, profileID int64) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM content_item_profiles WHERE content_item_id = ? AND profile_id = ?`,
		contentItemID, profileID)
	if err != nil {
		return fmt.Errorf("unassign profile: %w", err)
	}
	return nil
}

func queryShootProfiles(ctx context.Context, db *sql.DB, query string, args ...any) ([]ShootProfile, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query shoot profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ShootProfile
	for rows.Next() {
		p, err := scanShootProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shoot profiles: %w", err)
	}
	return out, nil
}

func scanShootProfile(sc rowScanner) (ShootProfile, error) {
	var (
		p            ShootProfile
		created, upd string
	)
	if err := sc.Scan(&p.ID, &p.ChannelID, &p.ChannelName, &p.Kind, &p.Name, &p.Details, &created, &upd); err != nil {
		return ShootProfile{}, err
	}
	p.CreatedAt = parseTS(created)
	p.UpdatedAt = parseTS(upd)
	return p, nil
}
