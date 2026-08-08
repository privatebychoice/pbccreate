package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), testLogger())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&got)
	switch err {
	case nil:
		return true
	case sql.ErrNoRows:
		return false
	default:
		t.Fatalf("tableExists(%q): %v", name, err)
		return false
	}
}

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	fsys := fstest.MapFS{
		"0002_b.sql": {Data: []byte(`CREATE TABLE b (id INTEGER PRIMARY KEY);`)},
		"0001_a.sql": {Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
	}
	db := openTestDB(t)

	// First run applies both migrations.
	n, err := Migrate(ctx, db, fsys, testLogger())
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if n != 2 {
		t.Fatalf("applied = %d, want 2", n)
	}
	for _, tbl := range []string{"a", "b"} {
		if !tableExists(t, db, tbl) {
			t.Errorf("table %q not created", tbl)
		}
	}

	// Second run is idempotent.
	n, err = Migrate(ctx, db, fsys, testLogger())
	if err != nil {
		t.Fatalf("Migrate (rerun): %v", err)
	}
	if n != 0 {
		t.Fatalf("rerun applied = %d, want 0", n)
	}

	// Versions are recorded.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 2 {
		t.Fatalf("schema_migrations rows = %d, want 2", count)
	}
}

func TestMigrateAtomicRollback(t *testing.T) {
	ctx := context.Background()
	// Second statement is invalid, so the whole migration must roll back and
	// table "good" must NOT persist.
	fsys := fstest.MapFS{
		"0001_bad.sql": {Data: []byte(`CREATE TABLE good (id INTEGER PRIMARY KEY); CREATE TABLE ;`)},
	}
	db := openTestDB(t)

	if _, err := Migrate(ctx, db, fsys, testLogger()); err == nil {
		t.Fatal("Migrate: expected error on invalid migration, got nil")
	}
	if tableExists(t, db, "good") {
		t.Error("failed migration was not rolled back: table \"good\" exists")
	}
}

func TestMigrateEmbedded(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if _, err := Migrate(ctx, db, MigrationsFS(), testLogger()); err != nil {
		t.Fatalf("Migrate embedded: %v", err)
	}
	for _, tbl := range []string{"channels", "content_items"} {
		if !tableExists(t, db, tbl) {
			t.Errorf("embedded migration did not create %q", tbl)
		}
	}
}
