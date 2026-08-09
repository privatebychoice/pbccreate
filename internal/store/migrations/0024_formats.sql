-- 0024_formats.sql — recurring formats (SPEC §5.14): named, reusable templates
-- (e.g. tutorial, review, essay) with a default type/mode and a default outline.
-- Selecting a format seeds a new content item, copying the outline in. A layer
-- above creator modes (§4). Default shot-list, thumbnail template, and checklist
-- bundling are follow-on work.

CREATE TABLE formats (
    id           INTEGER PRIMARY KEY,
    channel_id   INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    default_type TEXT NOT NULL DEFAULT 'video',
    default_mode TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_formats_channel ON formats (channel_id);

CREATE TABLE format_outline_segments (
    id             INTEGER PRIMARY KEY,
    format_id      INTEGER NOT NULL REFERENCES formats(id) ON DELETE CASCADE,
    position       INTEGER NOT NULL DEFAULT 0,
    title          TEXT NOT NULL,
    notes          TEXT NOT NULL DEFAULT '',
    target_seconds INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_format_outline_format ON format_outline_segments (format_id);
