package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// MigrationsFS returns the embedded migration set, rooted so that files appear
// as "0001_init.sql" etc.
func MigrationsFS() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		// Only possible on a programmer error (bad embed path); fail loudly.
		panic(fmt.Sprintf("store: embed migrations: %v", err))
	}
	return sub
}

// Migrate applies every pending "*.sql" migration in fsys, in filename order,
// each within its own transaction, and records applied versions in the
// schema_migrations table. It is idempotent: already-applied migrations are
// skipped. Returns the number newly applied.
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS, log *slog.Logger) (int, error) {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return 0, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	applied := 0
	for _, name := range names {
		ok, err := migrationApplied(ctx, db, name)
		if err != nil {
			return applied, err
		}
		if ok {
			continue
		}

		sqlText, err := fs.ReadFile(fsys, name)
		if err != nil {
			return applied, fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, db, name, string(sqlText)); err != nil {
			return applied, err
		}
		applied++
		log.Info("migration applied", "version", name)
	}

	log.Info("migrations complete", "applied", applied, "total", len(names))
	return applied, nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM schema_migrations WHERE version = ?`, version).Scan(&one)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
}

func applyMigration(ctx context.Context, db *sql.DB, version, sqlText string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
