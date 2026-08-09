package store

import (
	"context"
	"database/sql"
	"fmt"
)

// BackupTo writes a consistent standalone copy of the database to destPath using
// SQLite's VACUUM INTO. The result is a clean, fully self-contained database file
// (no separate WAL) that opens in any SQLite tool — a true full backup with no
// lock-in (SPEC §5.19). destPath must not already exist.
func BackupTo(ctx context.Context, db *sql.DB, destPath string) error {
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("backup (vacuum into): %w", err)
	}
	return nil
}
