package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// --- Sponsors ---

func (s *Server) handleSponsorsList(w http.ResponseWriter, r *http.Request) {
	s.renderSponsors(w, r, http.StatusOK, "")
}

func (s *Server) handleSponsorCreate(w http.ResponseWriter, r *http.Request) {
	sp, err := store.CreateSponsor(r.Context(), s.db,
		r.PostFormValue("name"), r.PostFormValue("contact"), r.PostFormValue("notes"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/sponsors/"+strconv.FormatInt(sp.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidSponsor):
		s.renderSponsors(w, r, http.StatusBadRequest, "Sponsor name is required.")
	default:
		s.log.Error("create sponsor", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSponsorDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderSponsorDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleSponsorUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateSponsor(r.Context(), s.db, id,
		r.PostFormValue("name"), r.PostFormValue("contact"), r.PostFormValue("notes"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/sponsors/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidSponsor):
		s.renderSponsorDetail(w, r, id, http.StatusBadRequest, "Sponsor name is required.")
	case errors.Is(err, store.ErrSponsorNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update sponsor", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSponsorDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.DeleteSponsor(r.Context(), s.db, id)
	if err != nil && !errors.Is(err, store.ErrSponsorNotFound) {
		s.log.Error("delete sponsor", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/sponsors", http.StatusSeeOther)
}

// --- Campaigns ---

func (s *Server) handleCampaignCreate(w http.ResponseWriter, r *http.Request) {
	sp, ok := s.requireSponsor(w, r)
	if !ok {
		return
	}
	c, err := store.CreateCampaign(r.Context(), s.db, store.Campaign{SponsorID: sp.ID, Name: r.PostFormValue("name")})
	switch {
	case err == nil:
		http.Redirect(w, r, campaignURL(sp.ID, c.ID), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidCampaign):
		http.Redirect(w, r, "/sponsors/"+strconv.FormatInt(sp.ID, 10), http.StatusSeeOther)
	default:
		s.log.Error("create campaign", "err", err, "sponsor", sp.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleCampaignEdit(w http.ResponseWriter, r *http.Request) {
	sp, c, ok := s.requireCampaign(w, r)
	if !ok {
		return
	}
	s.renderCampaign(w, r, sp, c)
}

func (s *Server) handleCampaignUpdate(w http.ResponseWriter, r *http.Request) {
	sp, c, ok := s.requireCampaign(w, r)
	if !ok {
		return
	}
	updated := campaignFromForm(r, c.ID, sp.ID)
	err := store.UpdateCampaign(r.Context(), s.db, updated)
	switch {
	case err == nil:
		http.Redirect(w, r, campaignURL(sp.ID, c.ID), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidCampaign):
		http.Error(w, "campaign name is required", http.StatusBadRequest)
	case errors.Is(err, store.ErrCampaignNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update campaign", "err", err, "campaign", c.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleCampaignDelete(w http.ResponseWriter, r *http.Request) {
	sp, c, ok := s.requireCampaign(w, r)
	if !ok {
		return
	}
	if err := store.DeleteCampaign(r.Context(), s.db, c.ID, sp.ID); err != nil && !errors.Is(err, store.ErrCampaignNotFound) {
		s.log.Error("delete campaign", "err", err, "campaign", c.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/sponsors/"+strconv.FormatInt(sp.ID, 10), http.StatusSeeOther)
}

// --- render helpers ---

func (s *Server) renderSponsors(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	sponsors, err := store.ListSponsors(r.Context(), s.db)
	if err != nil {
		s.log.Error("list sponsors", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Sponsors",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Sponsors":  sponsors,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "sponsors.html.tmpl", data); err != nil {
		s.log.Error("render sponsors", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderSponsorDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	sp, err := store.GetSponsor(r.Context(), s.db, id)
	if errors.Is(err, store.ErrSponsorNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get sponsor", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	campaigns, err := store.ListCampaigns(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("list campaigns", "err", err, "sponsor", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Sponsor: " + sp.Name,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Sponsor":   sp,
		"Campaigns": campaigns,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "sponsor_detail.html.tmpl", data); err != nil {
		s.log.Error("render sponsor detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderCampaign(w http.ResponseWriter, r *http.Request, sp store.Sponsor, c store.Campaign) {
	data := map[string]any{
		"Title":           "Campaign: " + c.Name,
		"Build":           buildinfo.Build,
		"CSRFToken":       csrfToken(r),
		"Sponsor":         sp,
		"Campaign":        c,
		"InvoiceStatuses": store.InvoiceStatuses,
		"PaymentStatuses": store.PaymentStatuses,
	}
	if err := s.tmpl.render(w, http.StatusOK, "campaign_edit.html.tmpl", data); err != nil {
		s.log.Error("render campaign", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// --- helpers ---

func campaignURL(sponsorID, campaignID int64) string {
	return "/sponsors/" + strconv.FormatInt(sponsorID, 10) + "/campaigns/" + strconv.FormatInt(campaignID, 10)
}

func campaignFromForm(r *http.Request, id, sponsorID int64) store.Campaign {
	c := store.Campaign{
		ID:            id,
		SponsorID:     sponsorID,
		Name:          r.PostFormValue("name"),
		StartsOn:      strings.TrimSpace(r.PostFormValue("starts_on")),
		EndsOn:        strings.TrimSpace(r.PostFormValue("ends_on")),
		TalkingPoints: r.PostFormValue("talking_points"),
		PromoCode:     strings.TrimSpace(r.PostFormValue("promo_code")),
		TrackingLink:  strings.TrimSpace(r.PostFormValue("tracking_link")),
		Currency:      strings.TrimSpace(r.PostFormValue("currency")),
		InvoiceStatus: r.PostFormValue("invoice_status"),
		PaymentStatus: r.PostFormValue("payment_status"),
	}
	if rs := strings.TrimSpace(r.PostFormValue("rate")); rs != "" {
		if v, err := strconv.ParseFloat(rs, 64); err == nil && v >= 0 {
			c.Rate, c.RateSet = v, true
		}
	}
	return c
}

// requireSponsor parses {id} and loads the sponsor (404/500 otherwise).
func (s *Server) requireSponsor(w http.ResponseWriter, r *http.Request) (store.Sponsor, bool) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return store.Sponsor{}, false
	}
	sp, err := store.GetSponsor(r.Context(), s.db, id)
	if errors.Is(err, store.ErrSponsorNotFound) {
		http.NotFound(w, r)
		return store.Sponsor{}, false
	}
	if err != nil {
		s.log.Error("get sponsor", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return store.Sponsor{}, false
	}
	return sp, true
}

// requireCampaign loads the sponsor and its campaign from {id}/{cid}.
func (s *Server) requireCampaign(w http.ResponseWriter, r *http.Request) (store.Sponsor, store.Campaign, bool) {
	sp, ok := s.requireSponsor(w, r)
	if !ok {
		return store.Sponsor{}, store.Campaign{}, false
	}
	cid, ok := pathInt64(w, r, "cid")
	if !ok {
		return store.Sponsor{}, store.Campaign{}, false
	}
	c, err := store.GetCampaign(r.Context(), s.db, cid, sp.ID)
	if errors.Is(err, store.ErrCampaignNotFound) {
		http.NotFound(w, r)
		return store.Sponsor{}, store.Campaign{}, false
	}
	if err != nil {
		s.log.Error("get campaign", "err", err, "campaign", cid)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return store.Sponsor{}, store.Campaign{}, false
	}
	return sp, c, true
}

// pathInt64 parses a path value as int64, writing 404 and returning ok=false on
// failure.
func pathInt64(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	return v, true
}
