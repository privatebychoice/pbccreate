package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// --- template management (top-level /checklists) ---

func (s *Server) handleChecklistsList(w http.ResponseWriter, r *http.Request) {
	s.renderChecklists(w, r, http.StatusOK, "")
}

func (s *Server) handleChecklistCreate(w http.ResponseWriter, r *http.Request) {
	t, err := store.CreateChecklistTemplate(r.Context(), s.db,
		r.PostFormValue("name"), r.PostFormValue("stage"), r.PostFormValue("description"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/checklists/"+strconv.FormatInt(t.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidChecklistTemplate):
		s.renderChecklists(w, r, http.StatusBadRequest, "A name is required.")
	default:
		s.log.Error("create checklist template", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleChecklistDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderChecklistDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleChecklistUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateChecklistTemplate(r.Context(), s.db, id,
		r.PostFormValue("name"), r.PostFormValue("stage"), r.PostFormValue("description"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/checklists/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidChecklistTemplate):
		s.renderChecklistDetail(w, r, id, http.StatusBadRequest, "A name is required.")
	case errors.Is(err, store.ErrChecklistTemplateNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update checklist template", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleChecklistDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteChecklistTemplate(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrChecklistTemplateNotFound) {
		s.log.Error("delete checklist template", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/checklists", http.StatusSeeOther)
}

func (s *Server) handleChecklistItemAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.AddTemplateItem(r.Context(), s.db, id, r.PostFormValue("text"))
	if err != nil && !errors.Is(err, store.ErrInvalidChecklistItem) {
		s.log.Error("add template item", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToChecklist(w, r, id)
}

func (s *Server) handleChecklistItemDelete(w http.ResponseWriter, r *http.Request) {
	id, itemID, ok := checklistAndItem(w, r)
	if !ok {
		return
	}
	err := store.DeleteTemplateItem(r.Context(), s.db, itemID, id)
	s.finishChecklistItem(w, r, id, err)
}

func (s *Server) handleChecklistItemMove(w http.ResponseWriter, r *http.Request) {
	id, itemID, ok := checklistAndItem(w, r)
	if !ok {
		return
	}
	err := store.MoveTemplateItem(r.Context(), s.db, itemID, id, r.PostFormValue("dir"))
	if errors.Is(err, store.ErrInvalidMove) {
		http.Error(w, "invalid move", http.StatusBadRequest)
		return
	}
	s.finishChecklistItem(w, r, id, err)
}

func (s *Server) finishChecklistItem(w http.ResponseWriter, r *http.Request, templateID int64, err error) {
	switch {
	case err == nil:
		s.redirectToChecklist(w, r, templateID)
	case errors.Is(err, store.ErrChecklistItemNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("checklist item op", "err", err, "template", templateID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// --- runs (on the content item) ---

// handleChecklistRunStart snapshots a template onto the content item.
func (s *Server) handleChecklistRunStart(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	templateID, _ := strconv.ParseInt(r.PostFormValue("template_id"), 10, 64)
	_, err := store.StartChecklistRun(r.Context(), s.db, id, templateID)
	switch {
	case err == nil, errors.Is(err, store.ErrChecklistTemplateNotFound):
		// A missing template selection just returns to the item.
		s.redirectToItem(w, r, id)
	default:
		s.log.Error("start checklist run", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleRunItemToggle(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	runID, _ := strconv.ParseInt(r.PathValue("runID"), 10, 64)
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Confirm the run belongs to this item before touching its item.
	if _, err := store.GetRun(r.Context(), s.db, runID, id); errors.Is(err, store.ErrChecklistRunNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		s.log.Error("get run for toggle", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.ToggleRunItem(r.Context(), s.db, itemID, runID); err != nil && !errors.Is(err, store.ErrRunItemNotFound) {
		s.log.Error("toggle run item", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

func (s *Server) handleChecklistRunDelete(w http.ResponseWriter, r *http.Request) {
	id, runID, ok := s.requireContentItemAndSub(w, r, "runID")
	if !ok {
		return
	}
	if err := store.DeleteRun(r.Context(), s.db, runID, id); err != nil && !errors.Is(err, store.ErrChecklistRunNotFound) {
		s.log.Error("delete checklist run", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// runView bundles a checklist run with its items and done count for the content
// detail page.
type runView struct {
	store.ChecklistRun
	Items     []store.RunItem
	DoneCount int
}

// checklistRunViews assembles the run views for a content item.
func (s *Server) checklistRunViews(r *http.Request, contentItemID int64) ([]runView, error) {
	runs, err := store.ListRuns(r.Context(), s.db, contentItemID)
	if err != nil {
		return nil, err
	}
	views := make([]runView, 0, len(runs))
	for _, run := range runs {
		items, err := store.ListRunItems(r.Context(), s.db, run.ID)
		if err != nil {
			return nil, err
		}
		done := 0
		for _, it := range items {
			if it.Done {
				done++
			}
		}
		views = append(views, runView{ChecklistRun: run, Items: items, DoneCount: done})
	}
	return views, nil
}

// --- render helpers ---

func (s *Server) renderChecklists(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	list, err := store.ListChecklistTemplates(r.Context(), s.db)
	if err != nil {
		s.log.Error("list checklist templates", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Checklists",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Templates": list,
		"Stages":    store.ChecklistStages,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "checklists.html.tmpl", data); err != nil {
		s.log.Error("render checklists", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderChecklistDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	t, err := store.GetChecklistTemplate(r.Context(), s.db, id)
	if errors.Is(err, store.ErrChecklistTemplateNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get checklist template", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items, err := store.ListTemplateItems(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list template items", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Checklist: " + t.Name,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Template":  t,
		"Items":     items,
		"Stages":    store.ChecklistStages,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "checklist_detail.html.tmpl", data); err != nil {
		s.log.Error("render checklist detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) redirectToChecklist(w http.ResponseWriter, r *http.Request, id int64) {
	http.Redirect(w, r, "/checklists/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func checklistAndItem(w http.ResponseWriter, r *http.Request) (templateID, itemID int64, ok bool) {
	templateID, ok = pathInt64(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	itemID, ok = pathInt64(w, r, "itemID")
	if !ok {
		return 0, 0, false
	}
	return templateID, itemID, true
}
