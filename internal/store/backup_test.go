package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupToProducesUsableCopy(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	if _, err := CreateContentItem(ctx, db, ch.ID, "video", "", "Backup me"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "copy.db")
	if err := BackupTo(ctx, db, dest); err != nil {
		t.Fatalf("BackupTo: %v", err)
	}
	if fi, err := os.Stat(dest); err != nil || fi.Size() == 0 {
		t.Fatalf("backup file missing or empty: %v", err)
	}

	// The copy is a standalone, queryable database with the seeded data.
	copyDB, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	defer func() { _ = copyDB.Close() }()

	var title string
	if err := copyDB.QueryRowContext(ctx, `SELECT title FROM content_items LIMIT 1`).Scan(&title); err != nil {
		t.Fatalf("query copy: %v", err)
	}
	if title != "Backup me" {
		t.Errorf("copy title = %q, want 'Backup me'", title)
	}
}

func TestChannelByName(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	made, _ := CreateChannel(ctx, db, "TUL", "youtube")

	got, err := ChannelByName(ctx, db, "tul") // case-insensitive
	if err != nil || got.ID != made.ID {
		t.Fatalf("ChannelByName: %v (%+v)", err, got)
	}
	if _, err := ChannelByName(ctx, db, "nope"); err != ErrChannelNotFound {
		t.Errorf("missing channel err = %v, want ErrChannelNotFound", err)
	}
}
