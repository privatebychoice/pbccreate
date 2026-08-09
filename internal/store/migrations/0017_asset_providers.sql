-- 0017_asset_providers.sql — operator-wide registry of 3rd-party media
-- subscriptions/services (SPEC §5.20). Global, NOT channel-scoped. Drives the
-- provider selector for license files (§5.11, later slice) and the optional
-- attributions.provider_id link. Stores a portal/account URL only — NEVER any
-- credential (plan-only, no secrets; §9).

CREATE TABLE asset_providers (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    service_type TEXT NOT NULL DEFAULT 'other'
                   CHECK (service_type IN ('music', 'sfx', 'stock', 'images', 'fonts', 'other')),
    website_url  TEXT NOT NULL DEFAULT '',
    plan_tier    TEXT NOT NULL DEFAULT '',
    billing_cycle TEXT NOT NULL DEFAULT '',
    renewal_on   TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'lapsed')),
    terms_notes  TEXT NOT NULL DEFAULT '',
    portal_url   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_asset_providers_name ON asset_providers (name COLLATE NOCASE);
