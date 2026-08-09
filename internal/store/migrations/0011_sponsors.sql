-- 0011_sponsors.sql — sponsors and their campaigns (SPEC §5.6). Placements
-- (linking a campaign to a content item) arrive in a later migration.

CREATE TABLE sponsors (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    contact    TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sponsor_campaigns (
    id             INTEGER PRIMARY KEY,
    sponsor_id     INTEGER NOT NULL REFERENCES sponsors(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    starts_on      TEXT NOT NULL DEFAULT '',
    ends_on        TEXT NOT NULL DEFAULT '',
    talking_points TEXT NOT NULL DEFAULT '',
    promo_code     TEXT NOT NULL DEFAULT '',
    tracking_link  TEXT NOT NULL DEFAULT '',
    rate           REAL,
    currency       TEXT NOT NULL DEFAULT '',
    invoice_status TEXT NOT NULL DEFAULT ''
                   CHECK (invoice_status IN ('', 'draft', 'sent')),
    payment_status TEXT NOT NULL DEFAULT ''
                   CHECK (payment_status IN ('', 'unpaid', 'partial', 'paid')),
    created_at     TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sponsor_campaigns_sponsor ON sponsor_campaigns (sponsor_id);
