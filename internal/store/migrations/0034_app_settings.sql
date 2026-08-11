-- Small key/value store for operator-set app settings that are not per-entity
-- (e.g. the DaVinci project root when not supplied via the environment). Keeping
-- it generic avoids a new table per setting.
CREATE TABLE app_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
