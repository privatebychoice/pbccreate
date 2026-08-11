package server

import (
	"net/http"
	"strconv"

	"go.privatebychoice.com/pbccreate/internal/resolve"
	"go.privatebychoice.com/pbccreate/internal/scaffold"
	"go.privatebychoice.com/pbccreate/internal/store"
)

// handleContentScaffold creates the DaVinci Resolve project folder tree for a
// content item under the configured project root (SPEC §8.1). It is a plain
// filesystem operation — no Resolve required. The project root comes from
// PBCCREATE_PROJECT_ROOT or the Data-page setting.
func (s *Server) handleContentScaffold(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requireContentItem(w, r)
	if !ok {
		return
	}
	item, err := store.GetContentItem(r.Context(), s.db, id)
	if err != nil {
		s.log.Error("scaffold: get item", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base, source, err := store.ResolveProjectRoot(r.Context(), s.db, s.cfg.ProjectRoot)
	if err != nil {
		s.log.Error("scaffold: resolve project root", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if source == store.ProjectRootUnset || base == "" {
		s.redirectScaffold(w, r, id, "noroot")
		return
	}

	docs, err := scaffold.Docs(r.Context(), s.db, item)
	if err != nil {
		s.log.Error("scaffold: build docs", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	res, err := (resolve.FSScaffolder{}).Scaffold(resolve.ScaffoldSpec{
		Base:        base,
		ProjectName: item.Title,
		Mode:        item.Mode,
		Template:    resolve.DefaultTemplate(item.Mode),
		Docs:        docs,
	})
	if err != nil {
		s.log.Error("scaffold: create tree", "err", err, "id", id, "base", base)
		s.redirectScaffold(w, r, id, "error")
		return
	}

	s.log.Info("scaffolded resolve project (web)", "id", id, "root", res.ProjectRoot,
		"dirs", len(res.Dirs), "docs", len(res.Docs))
	s.redirectScaffold(w, r, id, "ok")
}

func (s *Server) redirectScaffold(w http.ResponseWriter, r *http.Request, id int64, result string) {
	http.Redirect(w, r, "/content/"+strconv.FormatInt(id, 10)+"?scaffold="+result, http.StatusSeeOther)
}

// scaffoldResultNotice maps the ?scaffold= result code to a user-facing message.
func scaffoldResultNotice(code string) string {
	switch code {
	case "ok":
		return "Project folders scaffolded."
	case "error":
		return "Scaffolding failed — see the server log for details."
	default:
		return ""
	}
}
