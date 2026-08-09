package store

import (
	"context"
	"testing"
)

func TestChecklistTemplatesItemsAndRuns(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	ch, _ := CreateChannel(ctx, db, "TUL", "youtube")
	item, _ := CreateContentItem(ctx, db, ch.ID, "video", "", "Alpha")

	if _, err := CreateChecklistTemplate(ctx, db, "  ", "edit", ""); err != ErrInvalidChecklistTemplate {
		t.Errorf("empty name err = %v, want ErrInvalidChecklistTemplate", err)
	}

	// Unknown stage normalizes to pre_shoot.
	tpl, err := CreateChecklistTemplate(ctx, db, "Shoot day SOP", "bogus", "before rolling")
	if err != nil {
		t.Fatalf("CreateChecklistTemplate: %v", err)
	}
	if tpl.Stage != "pre_shoot" {
		t.Errorf("stage = %q, want normalized pre_shoot", tpl.Stage)
	}
	_ = UpdateChecklistTemplate(ctx, db, tpl.ID, "Shoot day SOP", "shoot_day", "before rolling")

	// Items: add three, reorder.
	if err := AddTemplateItem(ctx, db, tpl.ID, "  "); err != ErrInvalidChecklistItem {
		t.Errorf("empty item err = %v, want ErrInvalidChecklistItem", err)
	}
	_ = AddTemplateItem(ctx, db, tpl.ID, "Charge batteries")
	_ = AddTemplateItem(ctx, db, tpl.ID, "Check audio")
	_ = AddTemplateItem(ctx, db, tpl.ID, "Clear cards")
	items, _ := ListTemplateItems(ctx, db, tpl.ID)
	if len(items) != 3 || items[0].Text != "Charge batteries" {
		t.Fatalf("template items wrong: %+v", items)
	}
	if err := MoveTemplateItem(ctx, db, items[2].ID, tpl.ID, "up"); err != nil {
		t.Fatalf("MoveTemplateItem: %v", err)
	}
	items, _ = ListTemplateItems(ctx, db, tpl.ID)
	if items[1].Text != "Clear cards" {
		t.Fatalf("item order after move wrong: %+v", items)
	}

	// Summary reports the item count.
	summ, _ := ListChecklistTemplates(ctx, db)
	if len(summ) != 1 || summ[0].ItemCount != 3 {
		t.Fatalf("template summary wrong: %+v", summ)
	}

	// Start a run: it snapshots name/stage and copies items (all not done).
	runID, err := StartChecklistRun(ctx, db, item.ID, tpl.ID)
	if err != nil {
		t.Fatalf("StartChecklistRun: %v", err)
	}
	run, _ := GetRun(ctx, db, runID, item.ID)
	if run.Name != "Shoot day SOP" || run.Stage != "shoot_day" {
		t.Fatalf("run snapshot wrong: %+v", run)
	}
	runItems, _ := ListRunItems(ctx, db, runID)
	if len(runItems) != 3 || runItems[0].Done {
		t.Fatalf("run items wrong: %+v", runItems)
	}

	// Toggle one done.
	if err := ToggleRunItem(ctx, db, runItems[0].ID, runID); err != nil {
		t.Fatalf("ToggleRunItem: %v", err)
	}
	runItems, _ = ListRunItems(ctx, db, runID)
	if !runItems[0].Done {
		t.Errorf("run item not toggled: %+v", runItems[0])
	}

	// Editing the template afterward does not change the existing run (snapshot).
	_ = AddTemplateItem(ctx, db, tpl.ID, "Extra later item")
	runItems, _ = ListRunItems(ctx, db, runID)
	if len(runItems) != 3 {
		t.Errorf("run should be a snapshot, got %d items", len(runItems))
	}

	// Deleting the template leaves the run intact with its link cleared.
	if err := DeleteChecklistTemplate(ctx, db, tpl.ID); err != nil {
		t.Fatalf("DeleteChecklistTemplate: %v", err)
	}
	run, err = GetRun(ctx, db, runID, item.ID)
	if err != nil {
		t.Fatalf("run should survive template delete: %v", err)
	}
	if run.TemplateID != 0 {
		t.Errorf("template link should be cleared, got %d", run.TemplateID)
	}
	if items, _ := ListRunItems(ctx, db, runID); len(items) != 3 {
		t.Errorf("run items should survive template delete: %d", len(items))
	}

	// Delete the run.
	if err := DeleteRun(ctx, db, runID, item.ID); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
	if runs, _ := ListRuns(ctx, db, item.ID); len(runs) != 0 {
		t.Errorf("run not deleted: %d remain", len(runs))
	}
}
