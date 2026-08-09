-- 0019_publications.sql — per-platform publication records (SPEC §5.12). A
-- content item may be posted to more than one platform (YouTube is the primary
-- case), so this is one-to-many. Metrics are entered by hand; pbccreate makes no
-- network calls (YouTube API auto-fill is post-v1, §9.1/§13).

CREATE TABLE publications (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    platform        TEXT NOT NULL DEFAULT '',
    published_title TEXT NOT NULL DEFAULT '',
    external_id     TEXT NOT NULL DEFAULT '',
    url             TEXT NOT NULL DEFAULT '',
    output_file     TEXT NOT NULL DEFAULT '',
    posted_on       TEXT NOT NULL DEFAULT '',
    visibility      TEXT NOT NULL DEFAULT ''
                      CHECK (visibility IN ('', 'public', 'unlisted', 'private', 'scheduled')),
    tags_snapshot   TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_publications_item ON publications (content_item_id);
