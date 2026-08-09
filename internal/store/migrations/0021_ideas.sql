-- 0021_ideas.sql — lightweight pre-ContentItem idea log (SPEC §5.13). Ideas are
-- channel-scoped; ICE fields (impact/confidence/effort, 0 = unset) drive a
-- priority computed in Go (not stored, so it never goes stale). Promoting an
-- idea creates a ContentItem at status 'idea' and links it here; ON DELETE SET
-- NULL so deleting that item clears the link rather than deleting the idea.

CREATE TABLE ideas (
    id                       INTEGER PRIMARY KEY,
    channel_id               INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    title                    TEXT NOT NULL,
    note                     TEXT NOT NULL DEFAULT '',
    source                   TEXT NOT NULL DEFAULT '',
    ice_impact               INTEGER NOT NULL DEFAULT 0,
    ice_confidence           INTEGER NOT NULL DEFAULT 0,
    ice_effort               INTEGER NOT NULL DEFAULT 0,
    status                   TEXT NOT NULL DEFAULT 'open'
                               CHECK (status IN ('open', 'parked', 'dropped', 'promoted')),
    promoted_content_item_id INTEGER REFERENCES content_items(id) ON DELETE SET NULL,
    created_at               TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at               TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ideas_channel ON ideas (channel_id);
