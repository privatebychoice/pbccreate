-- 0004_shots.sql — ordered shot list per content item (SPEC §5.3).

CREATE TABLE shots (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    description     TEXT NOT NULL,
    scene           TEXT NOT NULL DEFAULT '',
    framing         TEXT NOT NULL DEFAULT '',
    camera          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'shot', 'selected')),
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_shots_item ON shots (content_item_id, position);
