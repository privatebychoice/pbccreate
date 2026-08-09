-- 0027_takes.sql — take/shot tracking (SPEC §5.15): record takes against a
-- shot-list row, mark the circle (good) take, rate, note, and optionally link the
-- catalogued media asset (§5.7) it corresponds to. Deleting that asset clears the
-- link (SET NULL); deleting the shot removes its takes (CASCADE).

CREATE TABLE takes (
    id             INTEGER PRIMARY KEY,
    shot_id        INTEGER NOT NULL REFERENCES shots(id) ON DELETE CASCADE,
    media_asset_id INTEGER REFERENCES media_assets(id) ON DELETE SET NULL,
    label          TEXT NOT NULL DEFAULT '',
    rating         INTEGER NOT NULL DEFAULT 0,
    circled        INTEGER NOT NULL DEFAULT 0,
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_takes_shot ON takes (shot_id);
