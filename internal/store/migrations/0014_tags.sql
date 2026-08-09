-- 0014_tags.sql — channel-scoped SEO tag library + per-item assignment (SPEC
-- §5.10). This is the outward-facing keyword namespace, distinct from the
-- internal project labels (§5.14, added later).

CREATE TABLE tags (
    id         INTEGER PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Case-insensitive uniqueness per channel (no duplicate keywords).
CREATE UNIQUE INDEX idx_tags_channel_name ON tags (channel_id, name COLLATE NOCASE);

CREATE TABLE content_item_tags (
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    tag_id          INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (content_item_id, tag_id)
);

CREATE INDEX idx_content_item_tags_item ON content_item_tags (content_item_id);
