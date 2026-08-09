-- 0005_media_assets.sql — catalogued media files per item (SPEC §5.7).

CREATE TABLE media_assets (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    shot_id         INTEGER REFERENCES shots(id) ON DELETE SET NULL,
    path            TEXT NOT NULL,
    filename        TEXT NOT NULL,
    kind            TEXT NOT NULL DEFAULT 'other' CHECK (kind IN ('video', 'audio', 'image', 'other')),
    size_bytes      INTEGER,
    mtime           TEXT,
    status          TEXT NOT NULL DEFAULT 'recorded'
                    CHECK (status IN ('to_shoot', 'recorded', 'imported', 'edited', 'used')),
    present         INTEGER NOT NULL DEFAULT 1,
    last_seen_at    TEXT,
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_media_assets_item ON media_assets (content_item_id);
