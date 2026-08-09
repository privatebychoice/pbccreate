package server

import (
	"fmt"
	"net/http"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleDescriptionSave upserts the item's description blocks.
func (s *Server) handleDescriptionSave(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	if _, err := store.SaveDescription(r.Context(), s.db, store.Description{
		ContentItemID: id,
		Intro:         r.PostFormValue("intro"),
		Chapters:      r.PostFormValue("chapters"),
		Links:         r.PostFormValue("links"),
		Sponsor:       r.PostFormValue("sponsor"),
		Hashtags:      r.PostFormValue("hashtags"),
		Disclosure:    r.PostFormValue("disclosure"),
	}); err != nil {
		s.log.Error("save description", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleDescriptionChapters regenerates the chapters block from the outline,
// preserving the other blocks.
func (s *Server) handleDescriptionChapters(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	segs, err := store.ListOutlineSegments(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list outline for chapters", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows, _ := buildOutline(segs)

	desc, err := store.GetDescription(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get description", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	desc.Chapters = generateChapters(rows)
	if _, err := store.SaveDescription(r.Context(), s.db, desc); err != nil {
		s.log.Error("save description chapters", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleDescriptionSponsor regenerates the sponsor blurb from the item's
// placements, preserving the other blocks.
func (s *Server) handleDescriptionSponsor(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	placements, err := store.ListPlacementsForItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list placements for blurb", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	desc, err := store.GetDescription(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("get description", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	desc.Sponsor = generateSponsorBlurb(placements)
	if _, err := store.SaveDescription(r.Context(), s.db, desc); err != nil {
		s.log.Error("save description sponsor", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// generateSponsorBlurb builds a description sponsor block from the item's
// placements (campaign talking points, promo code, tracking link).
func generateSponsorBlurb(placements []store.Placement) string {
	parts := make([]string, 0, len(placements))
	for _, p := range placements {
		var b strings.Builder
		fmt.Fprintf(&b, "Sponsored by %s.", p.SponsorName)
		if tp := strings.TrimSpace(p.TalkingPoints); tp != "" {
			b.WriteString("\n")
			b.WriteString(tp)
		}
		promo := strings.TrimSpace(p.PromoCode)
		link := strings.TrimSpace(p.TrackingLink)
		switch {
		case promo != "" && link != "":
			fmt.Fprintf(&b, "\nUse code %s: %s", promo, link)
		case promo != "":
			fmt.Fprintf(&b, "\nUse code %s", promo)
		case link != "":
			b.WriteString("\n")
			b.WriteString(link)
		}
		parts = append(parts, b.String())
	}
	return strings.Join(parts, "\n\n")
}

// generateChapters renders outline segments as YouTube-style chapter lines,
// e.g. "0:00 Intro". Returns "" when there are no segments.
func generateChapters(rows []outlineRow) string {
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(formatTimestamp(row.StartSeconds))
		b.WriteByte(' ')
		b.WriteString(row.Title)
	}
	return b.String()
}

// formatTimestamp renders seconds as "M:SS" (or "H:MM:SS" past an hour).
func formatTimestamp(sec int) string {
	if sec < 0 {
		sec = 0
	}
	h, m, s := sec/3600, (sec%3600)/60, sec%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
