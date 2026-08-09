package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// boardItem decorates a content item with its adjacent statuses so the board can
// render Back/Advance controls.
type boardItem struct {
	store.ContentItem
	PrevStatus string
	NextStatus string
}

// statusColumn is one pipeline column on the board.
type statusColumn struct {
	Status string
	Items  []boardItem
}

// handleContentBoard shows the create form and the pipeline board.
func (s *Server) handleContentBoard(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, http.StatusOK, "")
}

// handleContentCreate handles the new content-item form.
func (s *Server) handleContentCreate(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PostFormValue("channel_id"), 10, 64)
	_, err := store.CreateContentItem(r.Context(), s.db, channelID,
		r.PostFormValue("type"), r.PostFormValue("mode"), r.PostFormValue("title"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/content", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidContentItem):
		s.renderContent(w, r, http.StatusBadRequest, "A channel, a title, and a valid type are required.")
	default:
		s.log.Error("create content item", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderContent loads channels and content items, groups items into pipeline
// columns, and renders the board.
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	channels, err := store.ListChannels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list channels", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items, err := store.ListContentItems(r.Context(), s.db)
	if err != nil {
		s.log.Error("list content items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":       "Content",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"Types":       store.ContentTypes,
		"Modes":       store.CreatorModes,
		"Columns":     groupByStatus(items),
		"HasChannels": len(channels) > 0,
		"Error":       errMsg,
	}
	if err := s.tmpl.render(w, status, "content.html.tmpl", data); err != nil {
		s.log.Error("render content", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// groupByStatus buckets items into columns following the canonical status order,
// attaching each item's previous/next status for the Back/Advance controls.
// Items with an unrecognized status are dropped from the board (should not occur
// given the schema CHECK, but keeps the view robust).
func groupByStatus(items []store.ContentItem) []statusColumn {
	index := make(map[string]int, len(store.ContentStatuses))
	for i, st := range store.ContentStatuses {
		index[st] = i
	}

	byStatus := make(map[string][]boardItem, len(store.ContentStatuses))
	for _, it := range items {
		i, ok := index[it.Status]
		if !ok {
			continue
		}
		bi := boardItem{ContentItem: it}
		if i > 0 {
			bi.PrevStatus = store.ContentStatuses[i-1]
		}
		if i < len(store.ContentStatuses)-1 {
			bi.NextStatus = store.ContentStatuses[i+1]
		}
		byStatus[it.Status] = append(byStatus[it.Status], bi)
	}

	cols := make([]statusColumn, 0, len(store.ContentStatuses))
	for _, st := range store.ContentStatuses {
		cols = append(cols, statusColumn{Status: st, Items: byStatus[st]})
	}
	return cols
}

// handleContentDetail shows a single content item.
func (s *Server) handleContentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if errors.Is(err, store.ErrContentItemNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get content item", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	script, err := store.GetScript(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("get script", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	segments, err := store.ListOutlineSegments(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list outline segments", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows, total := buildOutline(segments)

	shots, err := store.ListShots(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list shots", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	assets, err := store.ListMediaAssets(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list media assets", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":               item.Title,
		"Build":               buildinfo.Build,
		"CSRFToken":           csrfToken(r),
		"Item":                item,
		"Statuses":            store.ContentStatuses,
		"Script":              script,
		"Segments":            rows,
		"OutlineTotalSeconds": total,
		"Shots":               shots,
		"ShotStatuses":        store.ShotStatuses,
		"Media":               assets,
		"MediaStatuses":       store.MediaStatuses,
		"MediaKinds":          store.MediaKinds,
	}
	if err := s.tmpl.render(w, http.StatusOK, "content_detail.html.tmpl", data); err != nil {
		s.log.Error("render content detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleContentStatus moves a content item to a new status and redirects back to
// the originating page.
func (s *Server) handleContentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = store.UpdateContentItemStatus(r.Context(), s.db, id, r.PostFormValue("status"))
	switch {
	case err == nil:
		http.Redirect(w, r, safeReturn(r.PostFormValue("return_to")), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidStatus):
		http.Error(w, "invalid status", http.StatusBadRequest)
	case errors.Is(err, store.ErrContentItemNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update content status", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleContentScriptSave upserts the script for a content item and returns to
// the item detail page.
func (s *Server) handleContentScriptSave(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}

	wpm, _ := strconv.Atoi(r.PostFormValue("wpm"))
	if _, err := store.SaveScript(r.Context(), s.db, id, r.PostFormValue("body"), wpm); err != nil {
		s.log.Error("save script", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// outlineRow decorates a segment with its cumulative start offset for display.
type outlineRow struct {
	store.OutlineSegment
	StartSeconds int
}

// buildOutline computes each segment's cumulative start offset and the total
// target duration.
func buildOutline(segs []store.OutlineSegment) (rows []outlineRow, total int) {
	start := 0
	for _, s := range segs {
		rows = append(rows, outlineRow{OutlineSegment: s, StartSeconds: start})
		start += s.TargetSeconds
	}
	return rows, start
}

// handleOutlineAdd appends a segment to a content item's outline.
func (s *Server) handleOutlineAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	target, _ := strconv.Atoi(r.PostFormValue("target_seconds"))
	_, err := store.AddOutlineSegment(r.Context(), s.db, id, r.PostFormValue("title"), r.PostFormValue("notes"), target)
	switch {
	case err == nil, errors.Is(err, store.ErrInvalidSegment):
		// On invalid input (empty title, normally blocked client-side) just
		// return to the page without adding.
		http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	default:
		s.log.Error("add outline segment", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleOutlineDelete removes a segment.
func (s *Server) handleOutlineDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	segID, err := strconv.ParseInt(r.PathValue("segID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = store.DeleteOutlineSegment(r.Context(), s.db, segID, id)
	switch {
	case err == nil:
		http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrOutlineSegmentNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("delete outline segment", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleOutlineMove reorders a segment up or down.
func (s *Server) handleOutlineMove(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	segID, err := strconv.ParseInt(r.PathValue("segID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = store.MoveOutlineSegment(r.Context(), s.db, segID, id, r.PostFormValue("dir"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidMove):
		http.Error(w, "invalid move", http.StatusBadRequest)
	case errors.Is(err, store.ErrOutlineSegmentNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("move outline segment", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// requireContentItem parses the {id} path value and verifies the content item
// exists, writing a 404/500 and returning ok=false otherwise.
func (s *Server) requireContentItem(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	if _, err := store.GetContentItem(r.Context(), s.db, id); errors.Is(err, store.ErrContentItemNotFound) {
		http.NotFound(w, r)
		return 0, false
	} else if err != nil {
		s.log.Error("get content item", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return 0, false
	}
	return id, true
}

// safeReturn restricts post-action redirects to in-app content paths, guarding
// against open redirects.
func safeReturn(to string) string {
	if strings.HasPrefix(to, "/content") {
		return to
	}
	return "/content"
}
