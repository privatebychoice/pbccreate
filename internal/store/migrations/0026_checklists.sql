-- 0026_checklists.sql — reusable per-stage checklists / SOPs (SPEC §5.15). A
-- template defines a checklist for a stage (pre-shoot, shoot-day, edit, publish);
-- starting a run snapshots the template's items onto a content item, each with
-- its own done state. Runs keep their own copy so editing a template later does
-- not change existing runs; deleting a template leaves runs intact (SET NULL).

CREATE TABLE checklist_templates (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    stage       TEXT NOT NULL DEFAULT 'pre_shoot'
                  CHECK (stage IN ('pre_shoot', 'shoot_day', 'edit', 'publish')),
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE checklist_template_items (
    id          INTEGER PRIMARY KEY,
    template_id INTEGER NOT NULL REFERENCES checklist_templates(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL DEFAULT 0,
    text        TEXT NOT NULL
);

CREATE INDEX idx_checklist_template_items_tpl ON checklist_template_items (template_id);

CREATE TABLE checklist_runs (
    id              INTEGER PRIMARY KEY,
    content_item_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    template_id     INTEGER REFERENCES checklist_templates(id) ON DELETE SET NULL,
    name            TEXT NOT NULL DEFAULT '',
    stage           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_checklist_runs_item ON checklist_runs (content_item_id);

CREATE TABLE checklist_run_items (
    id       INTEGER PRIMARY KEY,
    run_id   INTEGER NOT NULL REFERENCES checklist_runs(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    text     TEXT NOT NULL,
    done     INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_checklist_run_items_run ON checklist_run_items (run_id);
