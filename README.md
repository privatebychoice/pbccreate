# pbccreate

A **local-only, privacy-first content-planning tool for creators.** Plan videos,
shorts, blogs, and social posts end to end — scripts, outlines, shot lists, media,
thumbnails, descriptions, sponsors, licensing, publishing records, and DaVinci
Resolve project scaffolding — from a single self-contained binary that runs on
your own machine and makes no network calls.

pbccreate is a planning tool, not a publisher: it never posts on your behalf and
never phones home.

---

## Why it exists

Most creator tooling is cloud-hosted, ad-supported, and hungry for your data.
pbccreate takes the opposite stance:

- **Local-only.** A loopback web UI (default `http://127.0.0.1:8787`) backed by an
  on-disk SQLite database. Your plans never leave your computer.
- **Zero network egress by default.** Outbound networking is off unless explicitly
  enabled (`PBCCREATE_NETWORK`); v1 has no networked code paths at all.
- **No analytics, telemetry, tracking, or ads.** Ever.
- **Self-hosted front end.** All JavaScript, CSS, and fonts are embedded and
  served from the binary — no CDNs, no external frameworks, no third-party
  requests. A strict Content-Security-Policy is enforced.
- **Single static binary.** Pure Go, no CGO, no system SQLite — cross-compile once
  and copy it anywhere.

## Features

- **Pipeline board** — content items across idea → scripting → shooting → editing
  → scheduled → published → archived, with per-item detail organized into tabs.
- **Writing** — script editor (word count / read-time), outline/beats, and a shot
  list with shots linkable to beats and auto-labelled (e.g. `2A`).
- **Ideation** — idea log with ICE scoring, content pillars, working titles / A-B
  candidates, and a channel swipe file. Ideas can be linked to a pillar and
  promoted into the pipeline.
- **Media** — catalogue local files with missing-file detection, ffprobe metadata,
  and ffmpeg preview thumbnails; gear/location profiles; take tracking.
- **Thumbnails** — a WYSIWYG `<canvas>` editor with a deterministic server-side
  render authority (text + image layers, PNG/JPEG export).
- **Metadata** — descriptions (intro/chapters/links/sponsor/credits/disclosure/
  hashtags), SEO tags, project labels, and pillars.
- **Rights & business** — attributions/licensing credits, a music cue sheet,
  per-project license-file storage, external asset-provider subscriptions, and
  sponsor campaigns/placements/deliverables.
- **Release** — a pre-publish checklist, reusable stage checklists (SOPs),
  per-platform publication records, and manual retrospectives (no auto-analytics).
- **Blog repurposing** — turn a video into a linked blog draft and export portable
  Markdown with YAML front matter.
- **DaVinci Resolve integration** — scaffold a per-mode project folder tree (works
  everywhere), plus optional scripting to drive Resolve Studio.
- **Global search** and **data backup / bulk import**.

## Requirements

- **Go 1.26+** to build.
- Optional: **ffprobe** and **ffmpeg** for media metadata and preview thumbnails.
- Optional: **DaVinci Resolve Studio** and **Python 3** for Resolve *scripting*
  (folder *scaffolding* needs neither).

## Build

```bash
git clone https://github.com/privatebychoice/pbccreate.git
cd pbccreate
go build -o pbccreate ./cmd/pbccreate
```

For a release build, strip paths and inject the version:

```bash
go build -trimpath \
  -ldflags "-X go.privatebychoice.com/pbccreate/internal/buildinfo.Version=1.0.0" \
  -o pbccreate ./cmd/pbccreate
```

The result is a static, CGO-free binary you can cross-compile from one machine.

## Run

```bash
./pbccreate serve            # start the local web UI (default command)
```

Then open <http://127.0.0.1:8787>. Other commands:

```bash
./pbccreate version          # print version and build number
./pbccreate scaffold -item 1 # create a DaVinci Resolve project folder tree
./pbccreate script -item 1 -action ping   # drive Resolve Studio (Studio + Python 3)
./pbccreate help
```

## Configuration

All configuration is via environment variables with sensible defaults — nothing
machine-specific is compiled in. Paths default to XDG locations under `$HOME`.

| Variable | Default | Purpose |
|----------|---------|---------|
| `PBCCREATE_ADDR` | `127.0.0.1:8787` | Loopback listen address for the web UI |
| `PBCCREATE_DATA_DIR` | `$XDG_DATA_HOME/pbccreate` | SQLite database and local state |
| `PBCCREATE_CONFIG_DIR` | `$XDG_CONFIG_HOME/pbccreate` | Config files |
| `PBCCREATE_MEDIA_ROOTS` | *(none)* | `:`-separated roots that catalogued media must live under |
| `PBCCREATE_ASSET_ROOT` | *(none)* | Cross-project asset-library root |
| `PBCCREATE_PROJECT_ROOT` | *(none)* | Base folder that holds scaffolded Resolve project folders (also settable on the Data page) |
| `PBCCREATE_FFPROBE` | `ffprobe` | ffprobe executable (path or name on `PATH`) |
| `PBCCREATE_FFMPEG` | `ffmpeg` | ffmpeg executable |
| `PBCCREATE_PYTHON` | `python3` | Python 3 for the Resolve scripting helper |
| `PBCCREATE_NETWORK` | `false` | Master outbound-network switch (default-deny) |
| `PBCCREATE_LOG` | `info` | Log level: `debug` \| `info` \| `warn` \| `error` |

> Example values above (such as `127.0.0.1:8787`) are defaults, not requirements —
> override any of them for your environment.

## DaVinci Resolve integration

Two independent capabilities behind one interface:

- **Scaffolding** (all Resolve editions, no Resolve needed): creates a per-mode
  project folder tree under `PBCCREATE_PROJECT_ROOT`, optionally exporting the
  script and shot list into a `Docs` folder. Available from the CLI (`scaffold`)
  and the content editor's **Media** tab. Set the project root via
  `PBCCREATE_PROJECT_ROOT` or on the **Data** page.
- **Scripting** (Resolve **Studio** only): drives a running Resolve to create/open
  a project, import media into bins, and build a timeline from the shot list. The
  only supported route is `Go → python3 → DaVinciResolveScript` over a JSON
  boundary, using an embedded helper. It is runtime-detected and degrades
  gracefully when the prerequisites (`RESOLVE_SCRIPT_API`, `RESOLVE_SCRIPT_LIB`,
  `PYTHONPATH`, a usable Python 3, and Resolve running) are absent.

## Data & backups

Your database is a single SQLite file under `PBCCREATE_DATA_DIR`. The **Data** page
offers a one-click backup (a consistent standalone `.db` you can open in any SQLite
tool — no lock-in) and CSV bulk import for backfilling content items.

## Development

```bash
gofmt -l .        # formatting (should print nothing)
go vet ./...
go test ./...
```

Tests are standard `testing`-package unit tests plus HTTP handler tests; some
media tests are skipped automatically when ffmpeg/ffprobe are not installed.

## Dependencies

Core-first and deliberately small: only two direct modules, both pure Go with no
third-party transitive graph.

- [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) — pure-Go SQLite
  via `database/sql` (no CGO).
- [`golang.org/x/image`](https://pkg.go.dev/golang.org/x/image) — thumbnail
  scaling and text rendering.

Full vetting notes are in [`external_dependencies.md`](external_dependencies.md).
The full specification lives in [`docs/SPEC.md`](docs/SPEC.md).

## License

Licensed under the [Apache License 2.0](LICENSE).
