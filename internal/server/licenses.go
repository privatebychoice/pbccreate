package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// maxLicenseUpload caps a single license document. Kept under the global request
// body cap (maxRequestBody) so multipart overhead still fits.
const maxLicenseUpload = 15 << 20 // 15 MiB

// licenseExtAllowlist is the set of accepted extensions (SPEC §5.11). Files are
// stored opaquely and served download-only, so this is about intent, not parsing.
var licenseExtAllowlist = map[string]bool{
	".pdf": true, ".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".txt": true,
}

// handleLicenseUpload accepts a license document, validates its size/extension,
// copies it opaquely into the item's licenses folder under an app-generated
// stored name, and records its metadata (SPEC §5.11).
func (s *Server) handleLicenseUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size <= 0 {
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}
	if header.Size > maxLicenseUpload {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !licenseExtAllowlist[ext] {
		http.Error(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	providerID, _ := strconv.ParseInt(r.PostFormValue("provider_id"), 10, 64)

	// Insert metadata first to obtain an ID for the app-generated stored name.
	lf, err := store.CreateLicenseFile(r.Context(), s.db, store.LicenseFile{
		ContentItemID:    id,
		ProviderID:       providerID,
		OriginalFilename: cleanFilename(header.Filename),
		Description:      r.PostFormValue("description"),
		AppliesTo:        r.PostFormValue("applies_to"),
	})
	if err != nil {
		s.log.Error("create license file", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	storedName := fmt.Sprintf("lic-%d%s", lf.ID, ext)
	n, err := s.writeLicenseFile(id, storedName, file)
	if err != nil {
		s.log.Error("write license file", "err", err, "lic", lf.ID)
		// Roll back the orphaned metadata row so the list stays consistent.
		if derr := store.DeleteLicenseFile(r.Context(), s.db, lf.ID, id); derr != nil {
			s.log.Error("rollback license row", "err", derr, "lic", lf.ID)
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.SetLicenseStored(r.Context(), s.db, lf.ID, id, storedName, n); err != nil {
		s.log.Error("set license stored", "err", err, "lic", lf.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.redirectToItem(w, r, id)
}

// handleLicenseDownload streams a stored license file as a download only — never
// rendered inline — so an uploaded HTML/SVG can neither execute nor be sniffed
// into an active type (SPEC §9).
func (s *Server) handleLicenseDownload(w http.ResponseWriter, r *http.Request) {
	id, licID, ok := s.requireContentItemAndSub(w, r, "licID")
	if !ok {
		return
	}
	lf, err := store.GetLicenseFile(r.Context(), s.db, licID, id)
	if errors.Is(err, store.ErrLicenseFileNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get license file", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if lf.StoredName == "" {
		http.NotFound(w, r)
		return
	}
	path := s.licensePath(id, lf.StoredName)
	if !withinDir(path, s.licenseDir(id)) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", lf.OriginalFilename))
	if _, err := io.Copy(w, f); err != nil {
		s.log.Error("stream license file", "err", err, "lic", lf.ID)
	}
}

// handleLicenseDelete removes a license file and its bytes.
func (s *Server) handleLicenseDelete(w http.ResponseWriter, r *http.Request) {
	id, licID, ok := s.requireContentItemAndSub(w, r, "licID")
	if !ok {
		return
	}
	lf, err := store.GetLicenseFile(r.Context(), s.db, licID, id)
	if errors.Is(err, store.ErrLicenseFileNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get license file for delete", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.DeleteLicenseFile(r.Context(), s.db, licID, id); err != nil {
		s.log.Error("delete license file", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Best-effort removal of the bytes; a stale file is harmless but log it.
	if lf.StoredName != "" {
		path := s.licensePath(id, lf.StoredName)
		if withinDir(path, s.licenseDir(id)) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				s.log.Warn("remove license file bytes", "err", err, "lic", licID)
			}
		}
	}
	s.redirectToItem(w, r, id)
}

// writeLicenseFile copies the upload into the item's licenses folder and returns
// the number of bytes written.
func (s *Server) writeLicenseFile(itemID int64, storedName string, src io.Reader) (int64, error) {
	if err := os.MkdirAll(s.licenseDir(itemID), 0o700); err != nil {
		return 0, fmt.Errorf("create licenses dir: %w", err)
	}
	dst, err := os.Create(s.licensePath(itemID, storedName))
	if err != nil {
		return 0, fmt.Errorf("create license file: %w", err)
	}
	defer func() { _ = dst.Close() }()
	n, err := io.Copy(dst, src)
	if err != nil {
		return 0, fmt.Errorf("copy license file: %w", err)
	}
	return n, nil
}

func (s *Server) licenseDir(itemID int64) string {
	return filepath.Join(s.cfg.DataDir, "licenses", strconv.FormatInt(itemID, 10))
}

func (s *Server) licensePath(itemID int64, storedName string) string {
	return filepath.Join(s.licenseDir(itemID), filepath.Base(storedName))
}
