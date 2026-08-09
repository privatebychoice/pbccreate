-- 0020_retrospectives.sql — one retrospective per content item (SPEC §5.12):
-- structured reflection after publishing. Metrics are entered by hand;
-- pbccreate makes no network calls (no analytics fetch).

CREATE TABLE retrospectives (
    id                INTEGER PRIMARY KEY,
    content_item_id   INTEGER NOT NULL UNIQUE REFERENCES content_items(id) ON DELETE CASCADE,
    what_worked       TEXT NOT NULL DEFAULT '',
    to_improve        TEXT NOT NULL DEFAULT '',
    performance_notes TEXT NOT NULL DEFAULT '',
    reviewed_on       TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
