-- 0003_outline_segments.sql — ordered outline/layout segments per item (SPEC §5.2).

CREATE TABLE outline_segments (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    title           TEXT NOT NULL,
    notes           TEXT NOT NULL DEFAULT '',
    target_seconds  INTEGER,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_outline_segments_item ON outline_segments (content_item_id, position);
