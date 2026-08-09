-- 0012_sponsor_placements.sql — link a campaign to a content item, with a
-- deliverable checklist (SPEC §5.6).

CREATE TABLE sponsor_placements (
    id              INTEGER PRIMARY KEY,
    campaign_id     INTEGER NOT NULL REFERENCES sponsor_campaigns(id) ON DELETE CASCADE,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    deadline        TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (content_item_id, campaign_id)
);

CREATE INDEX idx_sponsor_placements_item ON sponsor_placements (content_item_id);
CREATE INDEX idx_sponsor_placements_campaign ON sponsor_placements (campaign_id);

CREATE TABLE sponsor_deliverables (
    id           INTEGER PRIMARY KEY,
    placement_id INTEGER NOT NULL REFERENCES sponsor_placements(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL,
    description  TEXT NOT NULL,
    done         INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sponsor_deliverables_placement ON sponsor_deliverables (placement_id);
