-- 0010_thumbnail_images.sql — uploaded images referenced by thumbnail layers
-- (SPEC §5.5). Scoped to a content item so images can be reused across its
-- thumbnails. Files live under the data dir; paths are derived from the id.

CREATE TABLE thumbnail_images (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    width           INTEGER NOT NULL,
    height          INTEGER NOT NULL,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_thumbnail_images_item ON thumbnail_images (content_item_id);
