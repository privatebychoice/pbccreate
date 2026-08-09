-- 0002_scripts.sql — one script (prose / voiceover) per content item (SPEC §5.1).

CREATE TABLE scripts (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL UNIQUE REFERENCES content_items(id) ON DELETE CASCADE,
    body            TEXT NOT NULL DEFAULT '',
    wpm             INTEGER NOT NULL DEFAULT 150,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
