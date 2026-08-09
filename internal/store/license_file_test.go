package store

import (
	"context"
	"testing"
)

func TestLicenseFileCRUD(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)

	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")
	prov, _ := CreateAssetProvider(ctx, db, AssetProvider{Name: "Artlist", ServiceType: "music"})

	if _, err := CreateLicenseFile(ctx, db, LicenseFile{ContentItemID: item.ID, OriginalFilename: "  "}); err != ErrInvalidLicenseFile {
		t.Errorf("empty filename err = %v, want ErrInvalidLicenseFile", err)
	}

	lf, err := CreateLicenseFile(ctx, db, LicenseFile{
		ContentItemID:    item.ID,
		ProviderID:       prov.ID,
		OriginalFilename: "cert.pdf",
		Description:      "annual certificate",
		AppliesTo:        "intro music",
	})
	if err != nil {
		t.Fatalf("CreateLicenseFile: %v", err)
	}
	// stored_name is empty until the bytes are written.
	if lf.StoredName != "" {
		t.Errorf("stored name = %q, want empty pre-write", lf.StoredName)
	}
	if err := SetLicenseStored(ctx, db, lf.ID, item.ID, "lic-1.pdf", 2048); err != nil {
		t.Fatalf("SetLicenseStored: %v", err)
	}

	list, err := ListLicenseFiles(ctx, db, item.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListLicenseFiles: %v (n=%d)", err, len(list))
	}
	got := list[0]
	if got.StoredName != "lic-1.pdf" || got.SizeBytes != 2048 {
		t.Errorf("stored metadata not applied: %+v", got)
	}
	if got.ProviderName != "Artlist" {
		t.Errorf("provider name join = %q, want Artlist", got.ProviderName)
	}

	// Deleting the provider clears the link (ON DELETE SET NULL), not the file.
	if err := DeleteAssetProvider(ctx, db, prov.ID); err != nil {
		t.Fatalf("DeleteAssetProvider: %v", err)
	}
	after, _ := GetLicenseFile(ctx, db, lf.ID, item.ID)
	if after.ProviderID != 0 || after.ProviderName != "" {
		t.Errorf("provider link not cleared after provider delete: %+v", after)
	}

	// Scoping: wrong content item is not found.
	if _, err := GetLicenseFile(ctx, db, lf.ID, item.ID+999); err != ErrLicenseFileNotFound {
		t.Errorf("cross-item get err = %v, want ErrLicenseFileNotFound", err)
	}

	if err := DeleteLicenseFile(ctx, db, lf.ID, item.ID); err != nil {
		t.Fatalf("DeleteLicenseFile: %v", err)
	}
	if _, err := GetLicenseFile(ctx, db, lf.ID, item.ID); err != ErrLicenseFileNotFound {
		t.Errorf("after delete err = %v, want ErrLicenseFileNotFound", err)
	}
}
