-- 0022_pillars.sql — content pillars/themes per channel (SPEC §5.13). Items are
-- assigned to pillars (many-to-many) and a coverage view counts items per pillar
-- to spot a neglected theme. Pillars carry a description, unlike the terse
-- project labels (§5.14); the two are distinct namespaces.

CREATE TABLE pillars (
    id          INTEGER PRIMARY KEY,
    channel_id  INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_pillars_channel_name ON pillars (channel_id, name COLLATE NOCASE);

CREATE TABLE content_item_pillars (
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    pillar_id       INTEGER NOT NULL REFERENCES pillars(id) ON DELETE CASCADE,
    PRIMARY KEY (content_item_id, pillar_id)
);

CREATE INDEX idx_content_item_pillars_item ON content_item_pillars (content_item_id);
