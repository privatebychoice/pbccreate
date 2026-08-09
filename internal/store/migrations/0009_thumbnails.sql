-- 0009_thumbnails.sql — thumbnail designs per content item (SPEC §5.5, §7).

CREATE TABLE thumbnails (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    name            TEXT NOT NULL DEFAULT '',
    canvas_json     TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_thumbnails_item ON thumbnails (content_item_id);
