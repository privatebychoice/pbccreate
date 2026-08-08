// Package store owns pbccreate's SQLite database: connection setup and schema
// migrations. It uses the pure-Go modernc.org/sqlite driver via database/sql so
// the binary cross-compiles with no CGO (see docs/SPEC.md §2, §10).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// Open opens (creating if needed) the SQLite database at path and returns a
// ready *sql.DB. Pragmas are applied per connection via the DSN.
//
// This is a single-user local app, so we cap the pool at one connection: it
// sidesteps SQLite writer-locking entirely while WAL keeps reads fast (§2).
func Open(ctx context.Context, path string, log *slog.Logger) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create data dir %q: %w", dir, err)
		}
	}

	dsn := "file:" + path + "?" + strings.Join([]string{
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
		"_pragma=foreign_keys(1)",
		"_pragma=synchronous(NORMAL)",
	}, "&")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	log.Info("sqlite store opened", "path", path)
	return db, nil
}
