-- 0007_media_preview.sql — cached preview-frame path per media asset (SPEC §5.7).

ALTER TABLE media_assets ADD COLUMN preview_path TEXT;
