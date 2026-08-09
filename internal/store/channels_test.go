package store

import (
	"context"
	"database/sql"
	"testing"
)

func migratedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t)
	if _, err := Migrate(context.Background(), db, MigrationsFS(), testLogger()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestCreateAndListChannels(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	if _, err := CreateChannel(ctx, db, "The Untracked Life", "youtube"); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := CreateChannel(ctx, db, "Privacy By Choice", "blog"); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	got, err := ListChannels(ctx, db)
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Ordered case-insensitively by name: "Privacy..." before "The Untracked...".
	if got[0].Name != "Privacy By Choice" || got[1].Name != "The Untracked Life" {
		t.Fatalf("unexpected order: %q, %q", got[0].Name, got[1].Name)
	}
	if got[0].Kind != "blog" {
		t.Errorf("kind = %q, want blog", got[0].Kind)
	}
	if got[0].CreatedAt.IsZero() {
		t.Error("CreatedAt not populated")
	}
}

func TestCreateChannelValidation(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	for _, name := range []string{"", "   ", "\t"} {
		if _, err := CreateChannel(ctx, db, name, "youtube"); err != ErrInvalidChannel {
			t.Errorf("CreateChannel(%q) err = %v, want ErrInvalidChannel", name, err)
		}
	}
	got, err := ListChannels(ctx, db)
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("invalid channels were persisted: %d rows", len(got))
	}
}

func TestCreateChannelTrimsName(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	c, err := CreateChannel(ctx, db, "  Spaced Name  ", "  youtube  ")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if c.Name != "Spaced Name" {
		t.Errorf("Name = %q, want trimmed", c.Name)
	}
	if c.Kind != "youtube" {
		t.Errorf("Kind = %q, want trimmed", c.Kind)
	}
}
