package server

import (
	"errors"
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

func (s *Server) handleAssetsList(w http.ResponseWriter, r *http.Request) {
	s.renderAssets(w, r, http.StatusOK, "")
}

func (s *Server) handleAssetCreate(w http.ResponseWriter, r *http.Request) {
	_, err := store.CreateLibraryAsset(r.Context(), s.db, libraryAssetFromForm(r, 0))
	switch {
	case err == nil:
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidLibraryAsset):
		s.renderAssets(w, r, http.StatusBadRequest, "A name is required.")
	default:
		s.log.Error("create library asset", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleAssetDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	s.renderAssetDetail(w, r, id, http.StatusOK, "")
}

func (s *Server) handleAssetUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	err := store.UpdateLibraryAsset(r.Context(), s.db, libraryAssetFromForm(r, id))
	switch {
	case err == nil:
		http.Redirect(w, r, "/assets/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, store.ErrInvalidLibraryAsset):
		s.renderAssetDetail(w, r, id, http.StatusBadRequest, "A name is required.")
	case errors.Is(err, store.ErrLibraryAssetNotFound):
		http.NotFound(w, r)
	default:
		s.log.Error("update library asset", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleAssetDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := store.DeleteLibraryAsset(r.Context(), s.db, id); err != nil && !errors.Is(err, store.ErrLibraryAssetNotFound) {
		s.log.Error("delete library asset", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets", http.StatusSeeOther)
}

// --- render helpers ---

func (s *Server) renderAssets(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	query := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	assets, err := store.ListLibraryAssets(r.Context(), s.db, query, kind)
	if err != nil {
		s.log.Error("list library assets", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	providers, err := store.ListAssetProviders(r.Context(), s.db)
	if err != nil {
		s.log.Error("list asset providers for library", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Assets",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Assets":    assets,
		"Kinds":     store.AssetLibraryKinds,
		"Providers": providers,
		"Query":     query,
		"KindF":     kind,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "assets.html.tmpl", data); err != nil {
		s.log.Error("render assets", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) renderAssetDetail(w http.ResponseWriter, r *http.Request, id int64, status int, errMsg string) {
	a, err := store.GetLibraryAsset(r.Context(), s.db, id)
	if errors.Is(err, store.ErrLibraryAssetNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get library asset", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	providers, err := store.ListAssetProviders(r.Context(), s.db)
	if err != nil {
		s.log.Error("list asset providers for library detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Asset: " + a.Name,
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Asset":     a,
		"Kinds":     store.AssetLibraryKinds,
		"Providers": providers,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "asset_detail.html.tmpl", data); err != nil {
		s.log.Error("render asset detail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// libraryAssetFromForm builds a LibraryAsset from the request form. Kind
// normalization happens in the store layer.
func libraryAssetFromForm(r *http.Request, id int64) store.LibraryAsset {
	providerID, _ := strconv.ParseInt(r.PostFormValue("provider_id"), 10, 64)
	return store.LibraryAsset{
		ID:         id,
		Kind:       r.PostFormValue("kind"),
		Name:       r.PostFormValue("name"),
		Path:       r.PostFormValue("path"),
		Tags:       r.PostFormValue("tags"),
		License:    r.PostFormValue("license"),
		ProviderID: providerID,
		Notes:      r.PostFormValue("notes"),
	}
}
