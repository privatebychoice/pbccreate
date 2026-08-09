// Command pbccreate is a local-only, privacy-first content-planning tool for
// creators. See docs/SPEC.md for the full specification.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/config"
	"go.privatebychoice.com/pbccreate/internal/resolve"
	"go.privatebychoice.com/pbccreate/internal/server"
	"go.privatebychoice.com/pbccreate/internal/store"
)

func main() {
	log := newLogger()

	// Dispatch: first non-flag argument is the subcommand; default is "serve".
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	var err error
	switch cmd {
	case "version":
		fmt.Println("pbccreate " + buildinfo.String())
	case "serve":
		err = runServe(log)
	case "scaffold":
		err = runScaffold(log, args)
	case "script":
		err = runScript(log, args)
	case "help", "-h", "--help":
		usage()
	default:
		log.Error("unknown command", "command", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Error("command failed", "command", cmd, "err", err)
		os.Exit(1)
	}
}

// runServe hosts the loopback web UI: it resolves configuration, opens and
// migrates the SQLite store, then serves until interrupted (SIGINT/SIGTERM).
func runServe(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Log(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPath := filepath.Join(cfg.DataDir, "pbccreate.db")
	db, err := store.Open(ctx, dbPath, log)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if _, err := store.Migrate(ctx, db, store.MigrationsFS(), log); err != nil {
		return err
	}

	srv, err := server.New(cfg, db, log)
	if err != nil {
		return err
	}
	return srv.Run(ctx)
}

// runScaffold creates a DaVinci Resolve project folder tree (docs/SPEC.md §8.1).
// With -item it derives the project name from the content item and exports the
// script and shot list into the Docs folder; the writable base comes from -root
// or PBCCREATE_PROJECT_ROOT. It also reports whether the optional Resolve
// scripting prerequisites are present (§8.2).
func runScaffold(log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	itemID := fs.Int64("item", 0, "content item ID to scaffold a Resolve project for")
	root := fs.String("root", "", "writable base directory (default: PBCCREATE_PROJECT_ROOT)")
	name := fs.String("name", "", "project folder name (default: derived from the item title)")
	docs := fs.Bool("docs", true, "export the script and shot list into the Docs folder (requires -item)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	base := *root
	if base == "" {
		base = cfg.ProjectRoot
	}
	if base == "" {
		return errors.New("no writable base directory: pass -root or set PBCCREATE_PROJECT_ROOT")
	}

	projectName := *name
	mode := ""
	var docFiles map[string]string

	if *itemID > 0 {
		ctx := context.Background()
		dbPath := filepath.Join(cfg.DataDir, "pbccreate.db")
		db, err := store.Open(ctx, dbPath, log)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if _, err := store.Migrate(ctx, db, store.MigrationsFS(), log); err != nil {
			return err
		}

		item, err := store.GetContentItem(ctx, db, *itemID)
		if err != nil {
			return err
		}
		mode = item.Mode
		if projectName == "" {
			projectName = item.Title
		}
		if *docs {
			docFiles, err = buildScaffoldDocs(ctx, db, item)
			if err != nil {
				return err
			}
		}
		log.Info("scaffolding from content item", "item", item.ID, "title", item.Title, "mode", item.Mode)
	}

	if strings.TrimSpace(projectName) == "" {
		return errors.New("no project name: pass -name or -item")
	}

	integ := resolve.New(cfg.Python)
	res, err := integ.Scaffolder().Scaffold(resolve.ScaffoldSpec{
		Base:        base,
		ProjectName: projectName,
		Mode:        mode,
		Template:    resolve.DefaultTemplate(mode),
		Docs:        docFiles,
	})
	if err != nil {
		return err
	}

	log.Info("scaffolded resolve project", "root", res.ProjectRoot, "dirs", len(res.Dirs), "docs", len(res.Docs))
	fmt.Println("Created project tree: " + res.ProjectRoot)
	for _, d := range res.Dirs {
		fmt.Println("  " + d)
	}
	for _, d := range res.Docs {
		fmt.Println("  " + d + "  (doc)")
	}

	if st := integ.Scripting(); st.Available {
		fmt.Println("\nResolve scripting: prerequisites detected (create/import/timeline lands in a later slice).")
	} else {
		fmt.Println("\nResolve scripting unavailable: " + st.Reason)
	}
	return nil
}

// buildScaffoldDocs renders the item's script and shot list into Markdown files
// for the scaffolded Docs folder. Empty sections are skipped.
func buildScaffoldDocs(ctx context.Context, db *sql.DB, item store.ContentItem) (map[string]string, error) {
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
		docs["shotlist.md"] = renderShotListMarkdown(item, shots)
	}

	return docs, nil
}

// renderShotListMarkdown formats a shot list as a numbered Markdown document.
func renderShotListMarkdown(item store.ContentItem, shots []store.Shot) string {
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

// runScript drives a running DaVinci Resolve Studio instance via the Python
// helper (docs/SPEC.md §8.2). It is optional and runtime-detected: if the
// scripting prerequisites are absent it reports the reason and does not attempt
// to script (the SQLite plan is untouched — Resolve is only a sink).
//
// Actions: ping (no item), create/import/timeline (require -item). Media bins and
// the timeline are derived from the item's catalogued media (§5.7) and shot list
// (§5.3).
func runScript(log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("script", flag.ContinueOnError)
	itemID := fs.Int64("item", 0, "content item ID to script from (required for create/import/timeline)")
	action := fs.String("action", "ping", "ping | create | import | timeline")
	project := fs.String("project", "", "Resolve project name (default: derived from the item title)")
	tlName := fs.String("timeline", "", "timeline name (default: project name)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	integ := resolve.New(cfg.Python)
	scripter, ok := integ.Scripter()
	if !ok {
		return errors.New(integ.Scripting().Reason)
	}

	ctx := context.Background()

	if *action == "ping" {
		resp, err := scripter.Ping(ctx)
		return reportScript(log, resp, err)
	}

	if *itemID <= 0 {
		return fmt.Errorf("action %q requires -item", *action)
	}

	dbPath := filepath.Join(cfg.DataDir, "pbccreate.db")
	db, err := store.Open(ctx, dbPath, log)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := store.Migrate(ctx, db, store.MigrationsFS(), log); err != nil {
		return err
	}

	item, err := store.GetContentItem(ctx, db, *itemID)
	if err != nil {
		return err
	}
	projectName := *project
	if projectName == "" {
		projectName = item.Title
	}

	switch *action {
	case "create":
		resp, err := scripter.CreateProject(ctx, resolve.ProjectSpec{Name: projectName})
		return reportScript(log, resp, err)
	case "import":
		assets, shots, err := loadMediaAndShots(ctx, db, item.ID)
		if err != nil {
			return err
		}
		bins := buildImportBins(assets, shots)
		if len(bins) == 0 {
			return errors.New("no catalogued media to import for this item")
		}
		log.Info("importing media to resolve", "item", item.ID, "bins", len(bins))
		resp, err := scripter.ImportMedia(ctx, bins)
		return reportScript(log, resp, err)
	case "timeline":
		assets, shots, err := loadMediaAndShots(ctx, db, item.ID)
		if err != nil {
			return err
		}
		name := *tlName
		if name == "" {
			name = projectName
		}
		tl := buildTimeline(name, item, shots, assets)
		log.Info("building resolve timeline", "item", item.ID, "clips", len(tl.Clips), "multicam", tl.Multicam)
		resp, err := scripter.BuildTimeline(ctx, tl)
		return reportScript(log, resp, err)
	default:
		return fmt.Errorf("unknown -action %q (want ping|create|import|timeline)", *action)
	}
}

// loadMediaAndShots fetches the item's catalogued media and shot list together.
func loadMediaAndShots(ctx context.Context, db *sql.DB, itemID int64) ([]store.MediaAsset, []store.Shot, error) {
	assets, err := store.ListMediaAssets(ctx, db, itemID)
	if err != nil {
		return nil, nil, err
	}
	shots, err := store.ListShots(ctx, db, itemID)
	if err != nil {
		return nil, nil, err
	}
	return assets, shots, nil
}

// reportScript renders a helper result: a transport error fails, a Resolve-side
// failure (OK=false) fails with its reason, otherwise the message is printed.
func reportScript(log *slog.Logger, resp resolve.Response, err error) error {
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("resolve reported failure: %s (code %s)", resp.Error, resp.Code)
	}
	log.Info("resolve scripting ok", "message", resp.Message)
	fmt.Println(resp.Message)
	return nil
}

// buildImportBins maps catalogued media into media-pool bins: video clips go to
// their shot's camera bin (or Footage when unlinked), audio to Audio, images to
// Graphics, everything else to Assets. Bin order follows first appearance.
func buildImportBins(assets []store.MediaAsset, shots []store.Shot) []resolve.Bin {
	camByShot := map[int64]string{}
	for _, s := range shots {
		if c := strings.TrimSpace(s.Camera); c != "" {
			camByShot[s.ID] = c
		}
	}

	var order []string
	clips := map[string][]string{}
	add := func(bin, path string) {
		if _, seen := clips[bin]; !seen {
			order = append(order, bin)
		}
		clips[bin] = append(clips[bin], path)
	}

	for _, a := range assets {
		path := strings.TrimSpace(a.Path)
		if path == "" {
			continue
		}
		switch a.Kind {
		case "video":
			if cam := camByShot[a.ShotID]; cam != "" {
				add(cam, path)
			} else {
				add("Footage", path)
			}
		case "audio":
			add("Audio", path)
		case "image":
			add("Graphics", path)
		default:
			add("Assets", path)
		}
	}

	bins := make([]resolve.Bin, 0, len(order))
	for _, name := range order {
		bins = append(bins, resolve.Bin{Name: name, Clips: clips[name]})
	}
	return bins
}

// buildTimeline orders shot-linked video clips into a timeline. For multi_cam
// items it flags Multicam and groups clip paths per camera to support downstream
// multicam sync (SPEC §8.2).
func buildTimeline(name string, item store.ContentItem, shots []store.Shot, assets []store.MediaAsset) resolve.Timeline {
	byShot := map[int64][]string{}
	for _, a := range assets {
		if a.Kind == "video" && a.ShotID != 0 {
			if p := strings.TrimSpace(a.Path); p != "" {
				byShot[a.ShotID] = append(byShot[a.ShotID], p)
			}
		}
	}

	var clips []string
	cameraBins := map[string][]string{}
	for _, s := range shots { // shots are returned in position order
		cam := strings.TrimSpace(s.Camera)
		for _, p := range byShot[s.ID] {
			clips = append(clips, p)
			if cam != "" {
				cameraBins[cam] = append(cameraBins[cam], p)
			}
		}
	}

	tl := resolve.Timeline{Name: name, Clips: clips}
	if item.Mode == "multi_cam" {
		tl.Multicam = true
		if len(cameraBins) > 0 {
			tl.CameraBins = cameraBins
		}
	}
	return tl
}

func usage() {
	fmt.Fprint(os.Stderr, `pbccreate — local content-planning tool for creators

Usage:
  pbccreate [command]

Commands:
  serve      Start the local web UI (default)
  scaffold   Create DaVinci Resolve project folders
  script     Drive DaVinci Resolve Studio (requires Studio + Python 3)
  version    Print version and build number
  help       Show this help
`)
}

// newLogger builds a structured stderr logger. Level is INFO by default and can
// be raised/lowered via PBCCREATE_LOG (debug|info|warn|error).
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("PBCCREATE_LOG"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
