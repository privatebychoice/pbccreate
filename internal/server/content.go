package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/media"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// boardItem decorates a content item with its adjacent statuses so the board can
// render Back/Advance controls, plus a sponsored flag for the badge.
type boardItem struct {
	store.ContentItem
	PrevStatus string
	NextStatus string
	HasSponsor bool
	Labels     []store.Label
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
	sponsored, err := store.ContentItemIDsWithPlacements(r.Context(), s.db)
	if err != nil {
		s.log.Error("list sponsored item ids", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	labelsByItem, err := store.LabelsByItem(r.Context(), s.db)
	if err != nil {
		s.log.Error("labels by item", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	allLabels, err := store.ListAllLabels(r.Context(), s.db)
	if err != nil {
		s.log.Error("list all labels", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Optional board filter by label.
	labelFilter, _ := strconv.ParseInt(r.URL.Query().Get("label"), 10, 64)
	if labelFilter > 0 {
		items = filterByLabel(items, labelsByItem, labelFilter)
	}

	data := map[string]any{
		"Title":       "Content",
		"Build":       buildinfo.Build,
		"CSRFToken":   csrfToken(r),
		"Channels":    channels,
		"Types":       store.ContentTypes,
		"Modes":       store.CreatorModes,
		"Columns":     groupByStatus(items, sponsored, labelsByItem),
		"HasChannels": len(channels) > 0,
		"AllLabels":   allLabels,
		"LabelFilter": labelFilter,
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
func groupByStatus(items []store.ContentItem, sponsored map[int64]bool, labelsByItem map[int64][]store.Label) []statusColumn {
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
		bi := boardItem{ContentItem: it, HasSponsor: sponsored[it.ID], Labels: labelsByItem[it.ID]}
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

// filterByLabel keeps only items that carry the given label id.
func filterByLabel(items []store.ContentItem, labelsByItem map[int64][]store.Label, labelID int64) []store.ContentItem {
	var out []store.ContentItem
	for _, it := range items {
		for _, l := range labelsByItem[it.ID] {
			if l.ID == labelID {
				out = append(out, it)
				break
			}
		}
	}
	return out
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

	// A scan redirect carries ?added=&skipped= to show a one-time notice.
	notice := ""
	if q := r.URL.Query(); q.Has("added") {
		added, _ := strconv.Atoi(q.Get("added"))
		skipped, _ := strconv.Atoi(q.Get("skipped"))
		notice = fmt.Sprintf("Imported %d file(s); skipped %d already catalogued.", added, skipped)
	}

	shots, err := store.ListShots(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list shots", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	takesByShot, err := store.TakesByShot(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("takes by shot", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shotViews := make([]shotView, 0, len(shots))
	for _, sh := range shots {
		shotViews = append(shotViews, shotView{Shot: sh, Takes: takesByShot[sh.ID]})
	}

	assets, err := store.ListMediaAssets(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list media assets", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	desc, err := store.GetDescription(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("get description", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	itemTags, err := store.ListTagsForItem(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list item tags", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	channelTags, err := store.ListTagsForChannel(r.Context(), s.db, item.ChannelID)
	if err != nil {
		s.log.Error("list channel tags", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	tagCSV := tagNamesCSV(itemTags)

	itemLabels, err := store.ListLabelsForItem(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list item labels", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	channelLabels, err := store.ListLabelsForChannel(r.Context(), s.db, item.ChannelID)
	if err != nil {
		s.log.Error("list channel labels", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	attributions, err := store.ListAttributions(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list attributions", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	licenseFiles, err := store.ListLicenseFiles(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list license files", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	musicCues, err := store.ListMusicCues(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list music cues", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	providers, err := store.ListAssetProviders(r.Context(), s.db)
	if err != nil {
		s.log.Error("list asset providers", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	publications, err := store.ListPublications(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list publications", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	retro, err := store.GetRetrospective(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("get retrospective", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	checklistTemplates, err := store.ListChecklistTemplates(r.Context(), s.db)
	if err != nil {
		s.log.Error("list checklist templates", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	checklistRuns, err := s.checklistRunViews(r, item.ID)
	if err != nil {
		s.log.Error("checklist run views", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	itemPillars, err := store.ListPillarsForItem(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list item pillars", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	channelPillars, err := store.ListPillarsForChannel(r.Context(), s.db, item.ChannelID)
	if err != nil {
		s.log.Error("list channel pillars", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	itemGear, err := store.ListProfilesForItem(r.Context(), s.db, item.ID, "gear")
	if err != nil {
		s.log.Error("list item gear", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	itemLocations, err := store.ListProfilesForItem(r.Context(), s.db, item.ID, "location")
	if err != nil {
		s.log.Error("list item locations", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	channelGear, err := store.ListProfilesForChannel(r.Context(), s.db, item.ChannelID, "gear")
	if err != nil {
		s.log.Error("list channel gear", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	channelLocations, err := store.ListProfilesForChannel(r.Context(), s.db, item.ChannelID, "location")
	if err != nil {
		s.log.Error("list channel locations", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	thumbs, err := store.ListThumbnails(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list thumbnails", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	placements, err := store.ListPlacementsForItem(r.Context(), s.db, item.ID)
	if err != nil {
		s.log.Error("list placements", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	placementViews := make([]placementView, 0, len(placements))
	for _, p := range placements {
		ds, err := store.ListDeliverables(r.Context(), s.db, p.ID)
		if err != nil {
			s.log.Error("list deliverables", "err", err, "placement", p.ID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		placementViews = append(placementViews, placementView{Placement: p, Deliverables: ds})
	}
	campaignOptions, err := store.ListCampaignOptions(r.Context(), s.db)
	if err != nil {
		s.log.Error("list campaign options", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	checklist, checklistReady := buildChecklist(checklistInput{
		Placements:   placementViews,
		Attributions: attributions,
		Thumbnails:   thumbs,
		RenderedDesc: desc.Render(),
		Tags:         itemTags,
		Publications: publications,
	})

	data := map[string]any{
		"Title":               item.Title,
		"Build":               buildinfo.Build,
		"CSRFToken":           csrfToken(r),
		"Item":                item,
		"Checklist":           checklist,
		"ChecklistReady":      checklistReady,
		"Statuses":            store.ContentStatuses,
		"Script":              script,
		"Segments":            rows,
		"OutlineTotalSeconds": total,
		"Shots":               shotViews,
		"ShotStatuses":        store.ShotStatuses,
		"Media":               assets,
		"MediaStatuses":       store.MediaStatuses,
		"MediaKinds":          store.MediaKinds,
		"ProbeAvailable":      media.ProbeAvailable(s.cfg.FFprobe),
		"ThumbAvailable":      media.ThumbAvailable(s.cfg.FFmpeg),
		"Notice":              notice,
		"Description":         desc,
		"DescriptionRendered": desc.Render(),
		"Thumbnails":          thumbs,
		"Placements":          placementViews,
		"CampaignOptions":     campaignOptions,
		"ItemTags":            itemTags,
		"ChannelTags":         channelTags,
		"TagList":             tagCSV,
		"TagLen":              len(tagCSV),
		"ItemLabels":          itemLabels,
		"ChannelLabels":       channelLabels,
		"LabelColors":         store.LabelColors,
		"ItemPillars":         itemPillars,
		"ChannelPillars":      channelPillars,
		"Attributions":        attributions,
		"AttributionKinds":    store.AttributionKinds,
		"LicenseFiles":        licenseFiles,
		"MusicCues":           musicCues,
		"Providers":           providers,
		"Publications":        publications,
		"Visibilities":        store.Visibilities,
		"Retrospective":       retro,
		"ChecklistTemplates":  checklistTemplates,
		"ChecklistRuns":       checklistRuns,
		"ItemGear":            itemGear,
		"ItemLocations":       itemLocations,
		"ChannelGear":         channelGear,
		"ChannelLocations":    channelLocations,
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
