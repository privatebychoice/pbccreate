-- 0001_init.sql — core tables (see docs/SPEC.md §3, §7).
-- Feature-specific tables are added by later migrations as those slices land.

CREATE TABLE channels (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE content_items (
    id               INTEGER PRIMARY KEY,
    channel_id       INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    type             TEXT NOT NULL CHECK (type IN ('video', 'short', 'blog', 'social')),
    mode             TEXT NOT NULL DEFAULT '' CHECK (mode IN ('', 'faceless', 'single_cam', 'multi_cam', 'obs')),
    title            TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'idea',
    target_seconds   INTEGER,
    derived_from_id  INTEGER REFERENCES content_items(id) ON DELETE SET NULL,
    last_export_path TEXT,
    created_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_content_items_channel ON content_items (channel_id);
CREATE INDEX idx_content_items_status  ON content_items (status);
CREATE INDEX idx_content_items_derived ON content_items (derived_from_id);
