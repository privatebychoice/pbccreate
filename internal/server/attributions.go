package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleAttributionAdd records a third-party asset attribution on the item
// (SPEC §5.11).
func (s *Server) handleAttributionAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	mediaID, _ := strconv.ParseInt(r.PostFormValue("media_asset_id"), 10, 64)
	providerID, _ := strconv.ParseInt(r.PostFormValue("provider_id"), 10, 64)
	_, err := store.CreateAttribution(r.Context(), s.db, store.Attribution{
		ContentItemID:         id,
		Name:                  r.PostFormValue("name"),
		Kind:                  r.PostFormValue("kind"),
		Provider:              r.PostFormValue("provider"),
		License:               r.PostFormValue("license"),
		LicenseID:             r.PostFormValue("license_id"),
		CreditText:            r.PostFormValue("credit_text"),
		SourceURL:             r.PostFormValue("source_url"),
		MediaAssetID:          mediaID,
		ProviderID:            providerID,
		IncludedInDescription: r.PostFormValue("included") != "",
	})
	switch {
	case err == nil, errors.Is(err, store.ErrInvalidAttribution):
		// Empty name is normally blocked client-side; just return to the page.
		s.redirectToItem(w, r, id)
	default:
		s.log.Error("create attribution", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleAttributionToggle flips whether an attribution feeds the credits block.
func (s *Server) handleAttributionToggle(w http.ResponseWriter, r *http.Request) {
	id, attrID, ok := s.requireContentItemAndSub(w, r, "attrID")
	if !ok {
		return
	}
	err := store.ToggleAttributionIncluded(r.Context(), s.db, attrID, id)
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrAttributionNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("toggle attribution", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleAttributionDelete removes an attribution from the item.
func (s *Server) handleAttributionDelete(w http.ResponseWriter, r *http.Request) {
	id, attrID, ok := s.requireContentItemAndSub(w, r, "attrID")
	if !ok {
		return
	}
	err := store.DeleteAttribution(r.Context(), s.db, attrID, id)
	switch {
	case err == nil:
		s.redirectToItem(w, r, id)
	case errors.Is(err, store.ErrAttributionNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("delete attribution", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleDescriptionCredits regenerates the credits block from the item's
// attributions marked for inclusion, preserving the other blocks.
func (s *Server) handleDescriptionCredits(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	attrs, err := store.ListAttributions(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list attributions for credits", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	desc, err := store.GetDescription(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get description", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	desc.Credits = generateCredits(attrs)
	if _, err := store.SaveDescription(r.Context(), s.db, desc); err != nil {
		s.log.Error("save description credits", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// generateCredits assembles a credits block from attributions marked for
// inclusion (SPEC §5.11). Each line uses the required credit text verbatim when
// present, otherwise a synthesized "Name by Provider (License)" line. Returns ""
// when nothing is marked for inclusion.
func generateCredits(attrs []store.Attribution) string {
	var lines []string
	for _, a := range attrs {
		if !a.IncludedInDescription {
			continue
		}
		if line := creditLine(a); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// creditLine builds a single credit line for one attribution.
func creditLine(a store.Attribution) string {
	if t := strings.TrimSpace(a.CreditText); t != "" {
		return t
	}
	var b strings.Builder
	b.WriteString(a.Name)
	if p := strings.TrimSpace(a.Provider); p != "" {
		fmt.Fprintf(&b, " by %s", p)
	}
	if l := strings.TrimSpace(a.License); l != "" {
		fmt.Fprintf(&b, " (%s)", l)
	}
	if u := strings.TrimSpace(a.SourceURL); u != "" {
		fmt.Fprintf(&b, " — %s", u)
	}
	return b.String()
}
