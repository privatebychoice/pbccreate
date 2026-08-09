-- 0016_attributions.sql — third-party asset attributions & licensing credits
-- (SPEC §5.11). Per content item: what external assets a video used and what
-- crediting each requires. Attributions marked included_in_description feed the
-- description credits block, so a new credits column is added to descriptions
-- (§5.4) for the assembled block to land in.

CREATE TABLE attributions (
    id                      INTEGER PRIMARY KEY,
    content_item_id         INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    kind                    TEXT NOT NULL DEFAULT 'other'
                              CHECK (kind IN ('music', 'sfx', 'stock', 'image', 'font', 'other')),
    provider                TEXT NOT NULL DEFAULT '',
    license                 TEXT NOT NULL DEFAULT '',
    license_id              TEXT NOT NULL DEFAULT '',
    credit_text             TEXT NOT NULL DEFAULT '',
    source_url              TEXT NOT NULL DEFAULT '',
    media_asset_id          INTEGER REFERENCES media_assets(id) ON DELETE SET NULL,
    included_in_description  INTEGER NOT NULL DEFAULT 1,
    created_at              TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_attributions_item ON attributions (content_item_id);

ALTER TABLE descriptions ADD COLUMN credits TEXT NOT NULL DEFAULT '';
