-- 0030_asset_library.sql — cross-project asset library (SPEC §5.16): an
-- operator-wide bank of reusable OWNED assets (B-roll, music, SFX, graphics,
-- brand elements), findable across all projects. Records a reference (path),
-- tags, license, and an optional provider link (§5.20). Plan-only: paths are
-- catalogued for recall, never opened by the app.

CREATE TABLE asset_library (
    id          INTEGER PRIMARY KEY,
    kind        TEXT NOT NULL DEFAULT 'other'
                  CHECK (kind IN ('b_roll', 'music', 'sfx', 'graphic', 'brand', 'other')),
    name        TEXT NOT NULL,
    path        TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '',
    license     TEXT NOT NULL DEFAULT '',
    provider_id INTEGER REFERENCES asset_providers(id) ON DELETE SET NULL,
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_asset_library_kind ON asset_library (kind);
