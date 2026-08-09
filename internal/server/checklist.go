package server

import (
	"fmt"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// checkState is a pre-publish check outcome: pass, fail (blocks readiness), or
// not-applicable (nothing required for this item, so it does not block).
type checkState string

const (
	checkOK   checkState = "ok"
	checkFail checkState = "fail"
	checkNA   checkState = "na"
)

// checkItem is one row of the pre-publish readiness view (SPEC §5.12).
type checkItem struct {
	Label  string
	State  checkState
	Detail string
}

// checklistInput bundles the already-loaded detail-page pieces the checklist
// aggregates, so building it needs no extra queries.
type checklistInput struct {
	Placements   []placementView
	Attributions []store.Attribution
	Thumbnails   []store.Thumbnail
	RenderedDesc string
	Tags         []store.Tag
	Publications []store.Publication
}

// buildChecklist computes the pre-publish readiness view and whether the item is
// ready (no failing checks; not-applicable checks do not block). It aggregates
// sponsor deliverables (§5.6), attributions (§5.11), thumbnail export (§5.5),
// and a tagged, rendered description (§5.4/§5.10). The publication output-file
// check and the mark-published/override action arrive with the publication
// record (§5.12).
func buildChecklist(in checklistInput) (items []checkItem, ready bool) {
	items = []checkItem{
		deliverableCheck(in.Placements),
		attributionCheck(in.Attributions),
		thumbnailCheck(in.Thumbnails),
		descriptionCheck(in.RenderedDesc, in.Tags),
		outputFileCheck(in.Publications),
	}
	ready = true
	for _, it := range items {
		if it.State == checkFail {
			ready = false
		}
	}
	return items, ready
}

// deliverableCheck passes when every sponsor deliverable across the item's
// placements is done; not-applicable when the item has none.
func deliverableCheck(placements []placementView) checkItem {
	total, done := 0, 0
	for _, p := range placements {
		for _, d := range p.Deliverables {
			total++
			if d.Done {
				done++
			}
		}
	}
	c := checkItem{Label: "Sponsor deliverables"}
	switch {
	case total == 0:
		c.State = checkNA
		c.Detail = "No sponsor deliverables to complete."
	case done == total:
		c.State = checkOK
		c.Detail = fmt.Sprintf("%d of %d complete.", done, total)
	default:
		c.State = checkFail
		c.Detail = fmt.Sprintf("%d of %d complete; %d outstanding.", done, total, total-done)
	}
	return c
}

// attributionCheck passes when every recorded attribution is marked for the
// credits block; not-applicable when the item has none.
func attributionCheck(attrs []store.Attribution) checkItem {
	included, notIncluded := 0, 0
	for _, a := range attrs {
		if a.IncludedInDescription {
			included++
		} else {
			notIncluded++
		}
	}
	c := checkItem{Label: "Attributions"}
	switch {
	case len(attrs) == 0:
		c.State = checkNA
		c.Detail = "No third-party attributions recorded."
	case notIncluded == 0:
		c.State = checkOK
		c.Detail = fmt.Sprintf("%d marked for the credits block.", included)
	default:
		c.State = checkFail
		c.Detail = fmt.Sprintf("%d not marked for the credits block.", notIncluded)
	}
	return c
}

// thumbnailCheck passes when at least one thumbnail design exists (each is
// exportable to PNG via its render endpoint, §5.5).
func thumbnailCheck(thumbs []store.Thumbnail) checkItem {
	c := checkItem{Label: "Thumbnail"}
	if len(thumbs) == 0 {
		c.State = checkFail
		c.Detail = "No thumbnail designed yet."
		return c
	}
	c.State = checkOK
	c.Detail = fmt.Sprintf("%d design(s) ready to export.", len(thumbs))
	return c
}

// outputFileCheck passes when at least one publication record has a rendered
// output file recorded (§5.12) — the master that actually ships.
func outputFileCheck(pubs []store.Publication) checkItem {
	c := checkItem{Label: "Output file"}
	for _, p := range pubs {
		if strings.TrimSpace(p.OutputFile) != "" {
			c.State = checkOK
			c.Detail = "Output file recorded on a publication."
			return c
		}
	}
	c.State = checkFail
	c.Detail = "No publication has an output file recorded yet."
	return c
}

// descriptionCheck passes when the rendered description is non-empty and the
// item has at least one tag (§5.4/§5.10).
func descriptionCheck(rendered string, tags []store.Tag) checkItem {
	c := checkItem{Label: "Description & tags"}
	hasDesc := strings.TrimSpace(rendered) != ""
	switch {
	case hasDesc && len(tags) > 0:
		c.State = checkOK
		c.Detail = fmt.Sprintf("Description ready with %d tag(s).", len(tags))
	case !hasDesc && len(tags) == 0:
		c.State = checkFail
		c.Detail = "Description is empty and no tags set."
	case !hasDesc:
		c.State = checkFail
		c.Detail = "Description is empty."
	default:
		c.State = checkFail
		c.Detail = "No tags set."
	}
	return c
}
