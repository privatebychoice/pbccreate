// Package scaffold builds the plan documents that accompany a DaVinci Resolve
// project scaffold (SPEC §8.1). It is shared by the CLI (`scaffold` subcommand)
// and the web UI so the two do not duplicate the export logic. The filesystem
// tree itself is created by internal/resolve.
package scaffold

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// Docs renders a content item's script and shot list into Markdown files for the
// scaffolded Docs folder (filename -> contents). Empty sections are skipped.
func Docs(ctx context.Context, db *sql.DB, item store.ContentItem) (map[string]string, error) {
	docs := map[string]string{}

	sc, err := store.GetScript(ctx, db, item.ID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sc.Body) != "" {
		docs["script.md"] = "# " + item.Title + " — Script\n\n" + strings.TrimSpace(sc.Body) + "\n"
	}

	shots, err := store.ListShots(ctx, db, item.ID)
	if err != nil {
		return nil, err
	}
	if len(shots) > 0 {
		docs["shotlist.md"] = renderShotList(item, shots)
	}

	return docs, nil
}

// renderShotList formats a shot list as a numbered Markdown document.
func renderShotList(item store.ContentItem, shots []store.Shot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — Shot list\n\n", item.Title)
	for _, s := range shots {
		fmt.Fprintf(&b, "%d. %s\n", s.Position, s.Description)
		if meta := shotMeta(s); meta != "" {
			fmt.Fprintf(&b, "   - %s\n", meta)
		}
		if strings.TrimSpace(s.Notes) != "" {
			fmt.Fprintf(&b, "   - Notes: %s\n", strings.TrimSpace(s.Notes))
		}
	}
	return b.String()
}

// shotMeta joins the non-empty scene/framing/camera fields of a shot.
func shotMeta(s store.Shot) string {
	var parts []string
	for _, p := range []struct{ label, val string }{
		{"Scene", s.Scene}, {"Framing", s.Framing}, {"Camera", s.Camera},
	} {
		if v := strings.TrimSpace(p.val); v != "" {
			parts = append(parts, p.label+": "+v)
		}
	}
	return strings.Join(parts, " · ")
}
