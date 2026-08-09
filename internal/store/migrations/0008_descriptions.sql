-- 0008_descriptions.sql — one templated description per content item (SPEC §5.4).

CREATE TABLE descriptions (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL UNIQUE REFERENCES content_items(id) ON DELETE CASCADE,
    intro           TEXT NOT NULL DEFAULT '',
    chapters        TEXT NOT NULL DEFAULT '',
    links           TEXT NOT NULL DEFAULT '',
    hashtags        TEXT NOT NULL DEFAULT '',
    disclosure      TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
