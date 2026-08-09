package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleProvidersList shows the create form and the provider registry (§5.20).
func (s *Server) handleProvidersList(w http.ResponseWriter, r *http.Request) {
	s.renderProviders(w, r, http.StatusOK, "")
}

func (s *Server) handleProviderCreate(w http.ResponseWriter, r *http.Request) {
	p, err := store.CreateAssetProvider(r.Context(), s.db, providerFromForm(r, 0))
	switch {
	case err == nil:
		http.Redirect(w, r, "/providers/"+strconv.FormatInt(p.ID, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidProvider):
		s.renderProviders(w, r, http.StatusBadRequest, "Provider name is required.")
	default:
		s.log.Error("create asset provider", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleProviderDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderProviderDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleProviderUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateAssetProvider(r.Context(), s.db, providerFromForm(r, id))
	switch {
	case err == nil:
		http.Redirect(w, r, "/providers/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidProvider):
		s.renderProviderDetail(w, r, id, http.StatusBadRequest, "Provider name is required.")
	case errors.Is(err, store.ErrProviderNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update asset provider", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleProviderDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteAssetProvider(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrProviderNotFound) {
		s.log.Error("delete asset provider", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/providers", http.StatusSeeOther)
}

// --- render helpers ---

func (s *Server) renderProviders(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	providers, err := store.ListAssetProviders(r.Context(), s.db)
	if err != nil {
		s.log.Error("list asset providers", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":        "Asset providers",
		"Build":        buildinfo.Build,
		"CSRFToken":    csrfToken(r),
		"Providers":    providers,
		"ServiceTypes": store.ServiceTypes,
		"Statuses":     store.ProviderStatuses,
		"Error":        errMsg,
	}
	if err := s.tmpl.render(w, status, "providers.html.tmpl", data); err != nil {
		s.log.Error("render providers", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderProviderDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	p, err := store.GetAssetProvider(r.Context(), s.db, id)
	if errors.Is(err, store.ErrProviderNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get asset provider", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":        "Provider: " + p.Name,
		"Build":        buildinfo.Build,
		"CSRFToken":    csrfToken(r),
		"Provider":     p,
		"ServiceTypes": store.ServiceTypes,
		"Statuses":     store.ProviderStatuses,
		"Error":        errMsg,
	}
	if err := s.tmpl.render(w, status, "provider_detail.html.tmpl", data); err != nil {
		s.log.Error("render provider detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// providerFromForm builds an AssetProvider from the request form. Field
// normalization (service type, status, trimming) happens in the store layer.
func providerFromForm(r *http.Request, id int64) store.AssetProvider {
	return store.AssetProvider{
		ID:           id,
		Name:         r.PostFormValue("name"),
		ServiceType:  r.PostFormValue("service_type"),
		WebsiteURL:   r.PostFormValue("website_url"),
		PlanTier:     r.PostFormValue("plan_tier"),
		BillingCycle: r.PostFormValue("billing_cycle"),
		RenewalOn:    r.PostFormValue("renewal_on"),
		Status:       r.PostFormValue("status"),
		TermsNotes:   r.PostFormValue("terms_notes"),
		PortalURL:    r.PostFormValue("portal_url"),
	}
}
