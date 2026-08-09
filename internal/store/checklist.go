package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Checklist errors and vocabulary (SPEC §5.15).
var (
	ErrInvalidChecklistTemplate  = errors.New("checklist name is required")
	ErrChecklistTemplateNotFound = errors.New("checklist template not found")
	ErrInvalidChecklistItem      = errors.New("checklist item text is required")
	ErrChecklistItemNotFound     = errors.New("checklist item not found")
	ErrChecklistRunNotFound      = errors.New("checklist run not found")
	ErrRunItemNotFound           = errors.New("checklist run item not found")

	// ChecklistStages is the production-stage vocabulary.
	ChecklistStages = []string{"pre_shoot", "shoot_day", "edit", "publish"}
)

func normChecklistStage(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if slices.Contains(ChecklistStages, s) {
		return s
	}
	return "pre_shoot"
}

// ChecklistTemplate is a reusable per-stage checklist/SOP.
type ChecklistTemplate struct {
	ID          int64
	Name        string
	Stage       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ChecklistTemplateSummary pairs a template with its item count for the list.
type ChecklistTemplateSummary struct {
	ChecklistTemplate
	ItemCount int
}

// ChecklistTemplateItem is one line of a template.
type ChecklistTemplateItem struct {
	ID         int64
	TemplateID int64
	Position   int
	Text       string
}

// ChecklistRun is a template snapshot applied to a content item. TemplateID is 0
// once the source template has been deleted.
type ChecklistRun struct {
	ID            int64
	ContentItemID int64
	TemplateID    int64
	Name          string
	Stage         string
	CreatedAt     time.Time
}

// RunItem is one checkable line in a run.
type RunItem struct {
	ID       int64
	RunID    int64
	Position int
	Text     string
	Done     bool
}

// --- templates ---

// CreateChecklistTemplate inserts a template (name required) and returns it.
func CreateChecklistTemplate(ctx context.Context, db *sql.DB, name, stage, description string) (ChecklistTemplate, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ChecklistTemplate{}, ErrInvalidChecklistTemplate
	}
	stage = normChecklistStage(stage)
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO checklist_templates (name, stage, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, stage, strings.TrimSpace(description), ts, ts)
	if err != nil {
		return ChecklistTemplate{}, fmt.Errorf("insert checklist template: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ChecklistTemplate{}, fmt.Errorf("checklist template last insert id: %w", err)
	}
	return GetChecklistTemplate(ctx, db, id)
}

// GetChecklistTemplate returns one template, or ErrChecklistTemplateNotFound.
func GetChecklistTemplate(ctx context.Context, db *sql.DB, id int64) (ChecklistTemplate, error) {
	var (
		t            ChecklistTemplate
		created, upd string
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, name, stage, description, created_at, updated_at FROM checklist_templates WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Stage, &t.Description, &created, &upd)
	if errors.Is(err, sql.ErrNoRows) {
		return ChecklistTemplate{}, ErrChecklistTemplateNotFound
	}
	if err != nil {
		return ChecklistTemplate{}, fmt.Errorf("get checklist template: %w", err)
	}
	t.CreatedAt = parseTS(created)
	t.UpdatedAt = parseTS(upd)
	return t, nil
}

// ListChecklistTemplates returns all templates with item counts, ordered by
// stage then name.
func ListChecklistTemplates(ctx context.Context, db *sql.DB) ([]ChecklistTemplateSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.name, t.stage, t.description, t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM checklist_template_items i WHERE i.template_id = t.id)
		FROM checklist_templates t
		ORDER BY t.stage, t.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query checklist templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ChecklistTemplateSummary
	for rows.Next() {
		var (
			ts           ChecklistTemplateSummary
			created, upd string
		)
		if err := rows.Scan(&ts.ID, &ts.Name, &ts.Stage, &ts.Description, &created, &upd, &ts.ItemCount); err != nil {
			return nil, fmt.Errorf("scan checklist template: %w", err)
		}
		ts.CreatedAt = parseTS(created)
		ts.UpdatedAt = parseTS(upd)
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checklist templates: %w", err)
	}
	return out, nil
}

// UpdateChecklistTemplate updates a template's fields (name required).
func UpdateChecklistTemplate(ctx context.Context, db *sql.DB, id int64, name, stage, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidChecklistTemplate
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE checklist_templates SET name = ?, stage = ?, description = ?, updated_at = ? WHERE id = ?`,
		name, normChecklistStage(stage), strings.TrimSpace(description), ts, id)
	if err != nil {
		return fmt.Errorf("update checklist template: %w", err)
	}
	return checkAffected(res, ErrChecklistTemplateNotFound)
}

// DeleteChecklistTemplate removes a template (its items cascade; existing runs
// keep their snapshot and their template link is cleared).
func DeleteChecklistTemplate(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM checklist_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete checklist template: %w", err)
	}
	return checkAffected(res, ErrChecklistTemplateNotFound)
}

// ListTemplateItems returns a template's items in order.
func ListTemplateItems(ctx context.Context, db *sql.DB, templateID int64) ([]ChecklistTemplateItem, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, template_id, position, text FROM checklist_template_items WHERE template_id = ? ORDER BY position, id`, templateID)
	if err != nil {
		return nil, fmt.Errorf("query template items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ChecklistTemplateItem
	for rows.Next() {
		var it ChecklistTemplateItem
		if err := rows.Scan(&it.ID, &it.TemplateID, &it.Position, &it.Text); err != nil {
			return nil, fmt.Errorf("scan template item: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate template items: %w", err)
	}
	return out, nil
}

// AddTemplateItem appends a line to a template (text required).
func AddTemplateItem(ctx context.Context, db *sql.DB, templateID int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrInvalidChecklistItem
	}
	var next int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM checklist_template_items WHERE template_id = ?`, templateID).Scan(&next); err != nil {
		return fmt.Errorf("next template item position: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO checklist_template_items (template_id, position, text) VALUES (?, ?, ?)`,
		templateID, next, text); err != nil {
		return fmt.Errorf("insert template item: %w", err)
	}
	return nil
}

// DeleteTemplateItem removes a line scoped to its template.
func DeleteTemplateItem(ctx context.Context, db *sql.DB, itemID, templateID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM checklist_template_items WHERE id = ? AND template_id = ?`, itemID, templateID)
	if err != nil {
		return fmt.Errorf("delete template item: %w", err)
	}
	return checkAffected(res, ErrChecklistItemNotFound)
}

// MoveTemplateItem reorders a template line up or down.
func MoveTemplateItem(ctx context.Context, db *sql.DB, itemID, templateID int64, dir string) error {
	err := moveOrdered(ctx, db, "checklist_template_items", itemID, templateID, dir)
	if errors.Is(err, errOrderedNotFound) {
		return ErrChecklistItemNotFound
	}
	return err
}

// --- runs ---

// StartChecklistRun snapshots a template onto a content item: it records the
// run (copying the template's name and stage) and copies each template item into
// a run item with its own done state.
func StartChecklistRun(ctx context.Context, db *sql.DB, contentItemID, templateID int64) (int64, error) {
	tpl, err := GetChecklistTemplate(ctx, db, templateID)
	if err != nil {
		return 0, err
	}
	items, err := ListTemplateItems(ctx, db, templateID)
	if err != nil {
		return 0, err
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO checklist_runs (content_item_id, template_id, name, stage, created_at) VALUES (?, ?, ?, ?, ?)`,
		contentItemID, templateID, tpl.Name, tpl.Stage, ts)
	if err != nil {
		return 0, fmt.Errorf("insert checklist run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("checklist run last insert id: %w", err)
	}
	for _, it := range items {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO checklist_run_items (run_id, position, text) VALUES (?, ?, ?)`,
			runID, it.Position, it.Text); err != nil {
			return 0, fmt.Errorf("copy run item: %w", err)
		}
	}
	return runID, nil
}

// ListRuns returns a content item's checklist runs, newest first.
func ListRuns(ctx context.Context, db *sql.DB, contentItemID int64) ([]ChecklistRun, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, content_item_id, COALESCE(template_id, 0), name, stage, created_at
		 FROM checklist_runs WHERE content_item_id = ? ORDER BY id DESC`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query checklist runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ChecklistRun
	for rows.Next() {
		var (
			run     ChecklistRun
			created string
		)
		if err := rows.Scan(&run.ID, &run.ContentItemID, &run.TemplateID, &run.Name, &run.Stage, &created); err != nil {
			return nil, fmt.Errorf("scan checklist run: %w", err)
		}
		run.CreatedAt = parseTS(created)
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checklist runs: %w", err)
	}
	return out, nil
}

// GetRun returns one run scoped to its content item.
func GetRun(ctx context.Context, db *sql.DB, runID, contentItemID int64) (ChecklistRun, error) {
	var (
		run     ChecklistRun
		created string
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, content_item_id, COALESCE(template_id, 0), name, stage, created_at
		 FROM checklist_runs WHERE id = ? AND content_item_id = ?`, runID, contentItemID).
		Scan(&run.ID, &run.ContentItemID, &run.TemplateID, &run.Name, &run.Stage, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return ChecklistRun{}, ErrChecklistRunNotFound
	}
	if err != nil {
		return ChecklistRun{}, fmt.Errorf("get checklist run: %w", err)
	}
	run.CreatedAt = parseTS(created)
	return run, nil
}

// ListRunItems returns a run's items in order.
func ListRunItems(ctx context.Context, db *sql.DB, runID int64) ([]RunItem, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, run_id, position, text, done FROM checklist_run_items WHERE run_id = ? ORDER BY position, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("query run items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []RunItem
	for rows.Next() {
		var it RunItem
		if err := rows.Scan(&it.ID, &it.RunID, &it.Position, &it.Text, &it.Done); err != nil {
			return nil, fmt.Errorf("scan run item: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run items: %w", err)
	}
	return out, nil
}

// ToggleRunItem flips a run item's done state, scoped to its run.
func ToggleRunItem(ctx context.Context, db *sql.DB, itemID, runID int64) error {
	res, err := db.ExecContext(ctx,
		`UPDATE checklist_run_items SET done = 1 - done WHERE id = ? AND run_id = ?`, itemID, runID)
	if err != nil {
		return fmt.Errorf("toggle run item: %w", err)
	}
	return checkAffected(res, ErrRunItemNotFound)
}

// DeleteRun removes a run (and its items, by cascade) scoped to its content item.
func DeleteRun(ctx context.Context, db *sql.DB, runID, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM checklist_runs WHERE id = ? AND content_item_id = ?`, runID, contentItemID)
	if err != nil {
		return fmt.Errorf("delete checklist run: %w", err)
	}
	return checkAffected(res, ErrChecklistRunNotFound)
}
