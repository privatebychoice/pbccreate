-- Link a shot to an outline segment (its "beat"), so shots can be organized and
-- labelled by beat and laid down in script order downstream (SPEC §5.2/§5.3/§8).
-- Nullable with ON DELETE SET NULL: deleting a beat unlinks its shots, it does
-- not delete them. SQLite accepts a REFERENCES clause on ADD COLUMN only for a
-- nullable column defaulting to NULL, which this is.
ALTER TABLE shots ADD COLUMN outline_segment_id INTEGER
    REFERENCES outline_segments(id) ON DELETE SET NULL;
