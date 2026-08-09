-- 0025_format_shots.sql — default shot-list for a recurring format (SPEC §5.14),
-- the shot-list half of the format bundle (outline arrived in 0024). Template
-- shots carry no status (that is per-item planning); seeding copies them into a
-- new content item at the default "planned" status.

CREATE TABLE format_shots (
    id          INTEGER PRIMARY KEY,
    format_id   INTEGER NOT NULL REFERENCES formats(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL DEFAULT 0,
    description TEXT NOT NULL,
    scene       TEXT NOT NULL DEFAULT '',
    framing     TEXT NOT NULL DEFAULT '',
    camera      TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_format_shots_format ON format_shots (format_id);
