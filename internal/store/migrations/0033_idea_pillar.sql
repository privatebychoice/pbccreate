-- Link an idea to a content pillar (its theme), so ideas can be organized by
-- pillar and the pillar carries over when the idea is promoted (SPEC §5.13).
-- Nullable with ON DELETE SET NULL: deleting a pillar unlinks its ideas rather
-- than deleting them. SQLite accepts REFERENCES on ADD COLUMN for a nullable
-- column defaulting to NULL, which this is.
ALTER TABLE ideas ADD COLUMN pillar_id INTEGER
    REFERENCES pillars(id) ON DELETE SET NULL;
