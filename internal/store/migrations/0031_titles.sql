-- 0031_titles.sql — working titles & A/B candidates (SPEC §5.13). Per item: a
-- set of candidate titles with one marked chosen. Per channel: a reusable swipe
-- file of title patterns that worked.

CREATE TABLE title_candidates (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    text            TEXT NOT NULL,
    chosen          INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_title_candidates_item ON title_candidates (content_item_id);

CREATE TABLE title_swipe (
    id         INTEGER PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    pattern    TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_title_swipe_channel ON title_swipe (channel_id);
