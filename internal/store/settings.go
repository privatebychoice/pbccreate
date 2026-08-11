package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Setting keys for the app_settings key/value store.
const SettingProjectRoot = "project_root"

// Sources for an effective project root (see ResolveProjectRoot).
const (
	ProjectRootEnv    = "env"
	ProjectRootStored = "stored"
	ProjectRootUnset  = "unset"
)

// GetSetting returns the stored value for key, or "" when it has never been set.
func GetSetting(ctx context.Context, db *sql.DB, key string) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return v, nil
}

// SetSetting upserts a trimmed value for key.
func SetSetting(ctx context.Context, db *sql.DB, key, value string) error {
	value = strings.TrimSpace(value)
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, ts)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// ResolveProjectRoot returns the effective Resolve project root and its source.
// The environment value (PBCCREATE_PROJECT_ROOT) takes precedence over the stored
// setting so an operator can override per deployment; source is one of
// ProjectRootEnv, ProjectRootStored, or ProjectRootUnset.
func ResolveProjectRoot(ctx context.Context, db *sql.DB, envValue string) (value, source string, err error) {
	if v := strings.TrimSpace(envValue); v != "" {
		return v, ProjectRootEnv, nil
	}
	stored, err := GetSetting(ctx, db, SettingProjectRoot)
	if err != nil {
		return "", "", err
	}
	if stored != "" {
		return stored, ProjectRootStored, nil
	}
	return "", ProjectRootUnset, nil
}
