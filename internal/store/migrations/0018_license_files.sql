-- 0018_license_files.sql — per-project license documents/certificates on disk
-- (SPEC §5.11) plus the optional provider link on attributions (§5.20). Files
-- live under <data_dir>/licenses/<content_item_id>/ with an app-generated
-- stored_name; the DB holds metadata only. Deleting a provider clears the link
-- (ON DELETE SET NULL) rather than cascading into content.

CREATE TABLE license_files (
    id                INTEGER PRIMARY KEY,
    content_item_id   INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    provider_id       INTEGER REFERENCES asset_providers(id) ON DELETE SET NULL,
    original_filename TEXT NOT NULL,
    stored_name       TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    applies_to        TEXT NOT NULL DEFAULT '',
    size_bytes        INTEGER NOT NULL DEFAULT 0,
    uploaded_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_license_files_item ON license_files (content_item_id);

-- Optional link from a description attribution to a registered provider (§5.20).
-- Nullable with a NULL default so SQLite accepts the REFERENCES clause on ADD
-- COLUMN; the free-text provider field remains for one-offs.
ALTER TABLE attributions ADD COLUMN provider_id INTEGER
    REFERENCES asset_providers(id) ON DELETE SET NULL;
