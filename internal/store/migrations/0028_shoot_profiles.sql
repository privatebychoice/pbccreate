-- 0028_shoot_profiles.sql — reusable gear and location profiles (SPEC §5.15).
-- Gear (camera/lens/mic/lighting/settings) and location/set profiles (power,
-- noise, golden-hour, permissions) are structurally identical — a channel-scoped
-- named profile with a text blob, assignable to content items — so they share one
-- table distinguished by `kind` instead of the two tables in the schema sketch.

CREATE TABLE shoot_profiles (
    id         INTEGER PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('gear', 'location')),
    name       TEXT NOT NULL,
    details    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_shoot_profiles_channel_kind_name
    ON shoot_profiles (channel_id, kind, name COLLATE NOCASE);

CREATE TABLE content_item_profiles (
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    profile_id      INTEGER NOT NULL REFERENCES shoot_profiles(id) ON DELETE CASCADE,
    PRIMARY KEY (content_item_id, profile_id)
);

CREATE INDEX idx_content_item_profiles_item ON content_item_profiles (content_item_id);
