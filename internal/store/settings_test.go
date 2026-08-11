package store

import (
	"context"
	"testing"
)

func TestSettingsGetSetAndResolveProjectRoot(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	// Unset key returns empty, no error.
	if v, err := GetSetting(ctx, db, SettingProjectRoot); err != nil || v != "" {
		t.Fatalf("GetSetting unset = %q, %v", v, err)
	}

	// Set (trims), then read back; upsert overwrites.
	if err := SetSetting(ctx, db, SettingProjectRoot, "  /projects  "); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if v, _ := GetSetting(ctx, db, SettingProjectRoot); v != "/projects" {
		t.Errorf("stored = %q, want trimmed /projects", v)
	}
	if err := SetSetting(ctx, db, SettingProjectRoot, "/other"); err != nil {
		t.Fatalf("SetSetting upsert: %v", err)
	}
	if v, _ := GetSetting(ctx, db, SettingProjectRoot); v != "/other" {
		t.Errorf("after upsert = %q, want /other", v)
	}

	// ResolveProjectRoot: env wins over stored.
	if val, src, _ := ResolveProjectRoot(ctx, db, "  /env/root "); val != "/env/root" || src != ProjectRootEnv {
		t.Errorf("env precedence = %q/%q, want /env/root/env", val, src)
	}
	// No env → stored value.
	if val, src, _ := ResolveProjectRoot(ctx, db, ""); val != "/other" || src != ProjectRootStored {
		t.Errorf("stored resolution = %q/%q, want /other/stored", val, src)
	}
	// No env, cleared stored → unset.
	if err := SetSetting(ctx, db, SettingProjectRoot, ""); err != nil {
		t.Fatalf("SetSetting clear: %v", err)
	}
	if val, src, _ := ResolveProjectRoot(ctx, db, ""); val != "" || src != ProjectRootUnset {
		t.Errorf("unset resolution = %q/%q, want empty/unset", val, src)
	}
}
