-- 0013_description_sponsor.sql — sponsor blurb block in descriptions (SPEC §5.4),
-- auto-fillable from the item's sponsor placements (§5.6).

ALTER TABLE descriptions ADD COLUMN sponsor TEXT NOT NULL DEFAULT '';
