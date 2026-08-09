-- 0015_project_labels.sql — internal organizational labels (SPEC §5.14),
-- a namespace SEPARATE from the outward SEO tags (§5.10). Color is a palette
-- key (see store.LabelColors), not arbitrary CSS, so the UI stays CSP-safe.

CREATE TABLE project_labels (
    id         INTEGER PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    color      TEXT NOT NULL DEFAULT 'blue',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_project_labels_channel_name ON project_labels (channel_id, name COLLATE NOCASE);

CREATE TABLE content_item_labels (
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    label_id        INTEGER NOT NULL REFERENCES project_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (content_item_id, label_id)
);

CREATE INDEX idx_content_item_labels_item ON content_item_labels (content_item_id);
