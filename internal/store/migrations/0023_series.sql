-- 0023_series.sql — series/playlist planner (SPEC §5.14). Group content items
-- into an ordered series with per-episode arc/continuity notes; maps to a
-- YouTube playlist at publish. Done-vs-planned is derived from each episode's
-- content-item status (published = done), not stored.

CREATE TABLE series (
    id          INTEGER PRIMARY KEY,
    channel_id  INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_series_channel ON series (channel_id);

CREATE TABLE series_items (
    id              INTEGER PRIMARY KEY,
    series_id       INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL DEFAULT 0,
    arc_notes       TEXT NOT NULL DEFAULT '',
    UNIQUE (series_id, content_item_id)
);

CREATE INDEX idx_series_items_series ON series_items (series_id);
