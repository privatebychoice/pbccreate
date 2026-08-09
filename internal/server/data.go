package server

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// maxImportBytes caps a bulk-import upload (kept under the global body cap).
const maxImportBytes = 8 << 20 // 8 MiB

// importResult summarizes a bulk import for the data page.
type importResult struct {
	Added   int
	Skipped []string // one human-readable reason per skipped row
}

func (s *Server) handleDataPage(w http.ResponseWriter, r *http.Request) {
	s.renderData(w, r, http.StatusOK, "", nil)
}

func (s *Server) renderData(w http.ResponseWriter, r *http.Request, status int, errMsg string, result *importResult) {
	data := map[string]any{
		"Title":     "Data",
		"Build":     buildinfo.Build,
		"CSRFToken": csrfToken(r),
		"Types":     store.ContentTypes,
		"Statuses":  store.ContentStatuses,
		"Result":    result,
		"Error":     errMsg,
	}
	if err := s.tmpl.render(w, status, "data.html.tmpl", data); err != nil {
		s.log.Error("render data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleBackupDownload streams a consistent standalone copy of the database.
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	tmpDir, err := os.MkdirTemp(s.cfg.DataDir, "backup-")
	if err != nil {
		s.log.Error("backup temp dir", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dest := filepath.Join(tmpDir, "pbccreate.db")
	if err := store.BackupTo(r.Context(), s.db, dest); err != nil {
		s.log.Error("backup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	f, err := os.Open(dest)
	if err != nil {
		s.log.Error("open backup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()

	name := "pbccreate-backup-" + time.Now().UTC().Format("20060102-150405") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	if _, err := io.Copy(w, f); err != nil {
		s.log.Error("stream backup", "err", err)
	}
}

// handleImportContent bulk-creates content items from an uploaded CSV with a
// header row: channel,type,mode,title,status (mode and status optional).
func (s *Server) handleImportContent(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		s.renderData(w, r, http.StatusBadRequest, "Choose a CSV file to import.", nil)
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size > maxImportBytes {
		s.renderData(w, r, http.StatusRequestEntityTooLarge, "CSV file is too large.", nil)
		return
	}

	cr := csv.NewReader(file)
	cr.FieldsPerRecord = -1 // tolerate ragged rows; we validate per row
	cr.TrimLeadingSpace = true

	rows, err := cr.ReadAll()
	if err != nil {
		s.renderData(w, r, http.StatusBadRequest, "Could not parse the CSV: "+err.Error(), nil)
		return
	}
	if len(rows) < 2 {
		s.renderData(w, r, http.StatusBadRequest, "The CSV needs a header row and at least one data row.", nil)
		return
	}

	cols := headerIndex(rows[0])
	if _, ok := cols["channel"]; !ok {
		s.renderData(w, r, http.StatusBadRequest, "Missing required 'channel' column.", nil)
		return
	}
	if _, ok := cols["title"]; !ok {
		s.renderData(w, r, http.StatusBadRequest, "Missing required 'title' column.", nil)
		return
	}

	result := &importResult{}
	for i, row := range rows[1:] {
		lineNo := i + 2 // 1-based, accounting for the header
		field := func(name string) string {
			if idx, ok := cols[name]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		channelName := field("channel")
		title := field("title")
		if channelName == "" || title == "" {
			result.Skipped = append(result.Skipped, fmt.Sprintf("row %d: channel and title are required", lineNo))
			continue
		}
		ch, err := store.ChannelByName(r.Context(), s.db, channelName)
		if errors.Is(err, store.ErrChannelNotFound) {
			result.Skipped = append(result.Skipped, fmt.Sprintf("row %d: no channel named %q", lineNo, channelName))
			continue
		}
		if err != nil {
			s.log.Error("import channel lookup", "err", err, "line", lineNo)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		typ := field("type")
		if typ == "" {
			typ = "video"
		}
		item, err := store.CreateContentItem(r.Context(), s.db, ch.ID, typ, field("mode"), title)
		if errors.Is(err, store.ErrInvalidContentItem) {
			result.Skipped = append(result.Skipped, fmt.Sprintf("row %d: invalid type/mode/title for %q", lineNo, title))
			continue
		}
		if err != nil {
			s.log.Error("import create item", "err", err, "line", lineNo)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Optional status (e.g. to backfill already-published videos).
		if st := field("status"); st != "" {
			if err := store.UpdateContentItemStatus(r.Context(), s.db, item.ID, st); errors.Is(err, store.ErrInvalidStatus) {
				result.Skipped = append(result.Skipped, fmt.Sprintf("row %d: created %q but status %q is invalid", lineNo, title, st))
			} else if err != nil {
				s.log.Error("import set status", "err", err, "line", lineNo)
			}
		}
		result.Added++
	}
	s.renderData(w, r, http.StatusOK, "", result)
}

// headerIndex maps lower-cased, trimmed header names to their column index.
func headerIndex(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, h := range header {
		out[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return out
}
