-- 0006_media_metadata.sql — technical media properties from ffprobe (SPEC §5.7).

ALTER TABLE media_assets ADD COLUMN duration_seconds INTEGER;
ALTER TABLE media_assets ADD COLUMN width INTEGER;
ALTER TABLE media_assets ADD COLUMN height INTEGER;
ALTER TABLE media_assets ADD COLUMN codec TEXT;
ALTER TABLE media_assets ADD COLUMN fps REAL;
ALTER TABLE media_assets ADD COLUMN container TEXT;
