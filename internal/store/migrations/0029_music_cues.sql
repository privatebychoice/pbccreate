-- 0029_music_cues.sql — per-item music cue sheet (SPEC §5.16): track list with
-- in/out points and license, useful for Content ID disputes and feeding the
-- attributions/credits (§5.11). A cue may link a registered provider (§5.20) and
-- the catalogued media asset (§5.7) it corresponds to; both clear on delete.

CREATE TABLE music_cues (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    provider_id     INTEGER REFERENCES asset_providers(id) ON DELETE SET NULL,
    media_asset_id  INTEGER REFERENCES media_assets(id) ON DELETE SET NULL,
    title           TEXT NOT NULL,
    artist          TEXT NOT NULL DEFAULT '',
    in_point        TEXT NOT NULL DEFAULT '',
    out_point       TEXT NOT NULL DEFAULT '',
    license         TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_music_cues_item ON music_cues (content_item_id);
