# pbccreate — Specification (v1)

**Status:** Ground truth for scope decisions. Living document; supersedes any
informal notes where they differ.

`pbccreate` (Privacy By Choice creator toolkit) is a **local-only, privacy-first**
content-planning tool for creators. It helps plan and organize video scripts,
outlines, shot lists, thumbnails, descriptions, media files, and sponsor
deliverables — and scaffolds DaVinci Resolve project folders on disk. It runs
entirely on the operator's machine, serves a web UI on loopback, stores
everything in a local SQLite database, and **makes no network calls**.

- Module path: `go.privatebychoice.com/pbccreate`
- Remote: `github.com/privatebychoice/pbccreate`
- Language/runtime: Go 1.26 (pure Go, no CGO — single cross-compiled binary)

---

## 1. Goals & non-goals

**Goals**

- Plan content end to end in one local tool: idea → script → outline/layout →
  shot list → media → thumbnail → description → publish-ready, tracked through a
  production pipeline.
- Support four **creator modes** — Faceless, Single Cam, Multi Cam, OBS — each
  with its own default outline, shot-list, media-bucket, and Resolve folder
  templates.
- Provide a self-hosted **thumbnail editor** (browser canvas for WYSIWYG editing;
  Go as the deterministic render authority for export).
- **Catalogue media files** per project/shot and detect moved/missing files.
- Track **sponsors and deliverables** (workflow-first, with optional financials).
- **Integrate with DaVinci Resolve**: scaffold project folders on disk from
  configurable per-mode templates, and drive Resolve *Studio* via its scripting
  API (create/open project, import media into bins, build a timeline from the shot
  list).
- **Repurpose video → blog**: derive an editable blog post from a video's
  script/outline/media and export it as portable Markdown + front matter.
- Stay **core-first**: standard library for nearly everything; a tiny, vetted,
  pure-Go dependency set (§10); external media/Resolve tooling invoked only at
  arm's length as separate processes.

**Non-goals (v1)**

- **No network egress of any kind.** No publishing, no OAuth, no platform APIs
  (YouTube/Mastodon/etc.), no telemetry, no analytics, no update checks, no
  external fonts/CDNs. Content is drafted and exported for the operator to post
  manually.
- No multi-user, RBAC, concurrent editing, or hosted/server mode. Single
  operator, local.
- No cloud sync, no mobile app.
- No video encoding/rendering inside `pbccreate` (it *invokes* `ffmpeg`/`ffprobe`
  for metadata and frame grabs; it is not an editor/encoder).

---

## 2. Execution model

One binary. Local, single-operator, loopback only.

- **`pbccreate serve`** (default when run with no subcommand) — starts an HTTP
  server bound to **`127.0.0.1`** on a configurable port and opens/serves the web
  admin UI. Loopback binding is the primary trust boundary.
- **`pbccreate scaffold`** — CLI entry point for Resolve folder scaffolding
  (§8), usable headless; the same operation is available from the UI.
- **`pbccreate version`** — prints version + build number (§11).

**Configuration** (no hardcoded environment values — §12): all paths and the
listen port come from flags/environment with sensible OS-appropriate defaults.

| Setting | Env / flag | Default (example) |
|---|---|---|
| Data dir (SQLite DB) | `PBCCREATE_DATA_DIR` | XDG data dir, e.g. `$HOME/.local/share/pbccreate` |
| Config dir | `PBCCREATE_CONFIG_DIR` | XDG config dir |
| Media root(s) | `PBCCREATE_MEDIA_ROOTS` | *(unset — operator configures per install)* |
| Listen address | `PBCCREATE_ADDR` | `127.0.0.1:8787` |
| `ffprobe` path | `PBCCREATE_FFPROBE` | resolved from `PATH` |
| `ffmpeg` path | `PBCCREATE_FFMPEG` | resolved from `PATH` |
| `python3` path (Resolve helper) | `PBCCREATE_PYTHON` | resolved from `PATH` |

*(Default paths above are examples; nothing environment-specific is compiled in.)*

**Storage:** a single SQLite database via `database/sql` + `modernc.org/sqlite`
(pure Go, §10). WAL mode, `busy_timeout` set, single-writer discipline
(`SetMaxOpenConns(1)` or serialized writes) — appropriate for one local user.

---

## 3. Domain model

```
Channel (brand, e.g. a YouTube channel / blog)
└── ContentItem (video | blog | social)      ← the central unit
    ├── derived_from → ContentItem (nullable; e.g. a blog repurposed from a video, §5.9)
    ├── mode: faceless | single_cam | multi_cam | obs   (videos)
    ├── status: idea → scripting → shooting → editing → scheduled → published → archived
    ├── Script            (beats/sections; VO for faceless)
    ├── Outline/Layout    (ordered segments + timing; scene composition for OBS/multicam)
    ├── ShotList          (ordered shots; columns vary by mode)
    ├── Thumbnail(s)      (layered designs → exported PNG/JPG)
    ├── Description       (templated blocks; chapters, links, sponsor blurb, disclosure)
    ├── MediaAsset(s)     (catalogued files linked to item/shot)
    ├── SponsorPlacement(s)  (deliverables from a campaign, tied to this item)
    └── Task(s)           (production checklist)

Sponsor ─< SponsorCampaign ─< SponsorPlacement >─ ContentItem
```

- **Channel** — a brand/destination (the operator may run several, e.g. distinct
  YouTube channels/blogs). Every `ContentItem` belongs to one channel; channels
  carry default templates (description blocks, brand kit) inherited by their
  items.
- **ContentItem** — the unit of planning. Carries type, creator mode (videos),
  and a **pipeline status** used for a kanban/board view.

---

## 4. Creator modes

Modes are **template presets**, not separate code paths. Selecting a mode seeds
the item's default outline, shot-list shape, media buckets, and Resolve folder
template (§8). All templates are editable per item and per channel; defaults ship
with the app and can be customized (stored in config, not hardcoded).

| Mode | Script emphasis | Shot list shape | Media buckets | Resolve folders |
|---|---|---|---|---|
| **Faceless** | Full voiceover script | B-roll / visual cues, screen recordings, on-screen text | VO, B-roll, screen-caps, music, SFX | VO audio, B-roll, screen-recordings, graphics |
| **Single Cam** | Talking-head script + B-roll cues | A-roll takes (framing/lens), B-roll | A-cam, B-roll, audio | A-Cam, B-Roll, audio |
| **Multi Cam** | Script + per-angle direction | Per-camera rows, sync references | Cam A/B/C…, audio | Cam A/B/C…, multicam-sync, audio |
| **OBS** | Segment/talking points | Scene list + source composition (screen/webcam/overlays) | OBS recordings per scene, overlays | OBS recordings, scenes, overlays |

"**Layout**" (from the goal statement) is modeled as the **Outline/Layout**: the
ordered segment structure (hook, intro, body sections, CTA, outro) with timing
estimates, plus — for OBS and Multi Cam — scene/source composition notes. It
doubles as the structural outline for blog items.

---

## 5. Feature specifications

### 5.1 Script planning
- Sectioned/beat-based editor (intro, hook, segments, CTA, outro). Plain-text /
  lightweight markup, stored per `ContentItem`. Word count + estimated read/VO
  duration (configurable words-per-minute).
- For **Faceless**, the script *is* the voiceover; segments can carry a paired
  visual cue that seeds shot-list/B-roll rows.

### 5.2 Outline / Layout
- Ordered segments with title, notes, target duration; running total vs a target
  length. Segment timings feed **auto-generated chapter timestamps** for the
  description (§5.4). For OBS/Multi Cam, each segment can hold scene/source
  composition notes.

### 5.3 Shot list
- Ordered, mode-shaped rows (see §4). Common fields: shot #, description,
  location/scene, framing/angle, camera (multicam), status (planned/shot/
  selected), linked media asset(s), notes. Reorderable; exportable to the
  Resolve `Docs` folder as plain text/CSV during scaffolding (§8).

### 5.4 Descriptions
- Per-channel **template** composed of blocks: intro, links, **chapters**
  (auto-filled from outline timings, editable), **sponsor blurb** (pulled from an
  attached campaign, §5.6), hashtags/tags, and a **disclosure/affiliate** block.
- Rendered to a copy-ready plain-text description. No links are fetched or
  contacted; this is text assembly only.
- *Future (Phase 2):* optionally run description links through
  [`pbc-classification`](https://github.com/privatebychoice/pbc-classification)
  to flag the privacy posture of outbound links before the operator posts them.

### 5.5 Thumbnail editor
- **WYSIWYG canvas** (self-hosted vanilla JS `<canvas>`, no framework/CDN) with a
  simple **layer model**: background (solid/gradient/image), image layers
  (imported raster + brand logos), text layers (font, size, fill, stroke,
  shadow), and basic shapes. Drag / resize / rotate; z-order; snapping and
  **YouTube safe-zone guides** (1280×720, title/duration overlap zones).
- **Brand kit** per channel: self-hosted fonts (TTF/OTF), color palette, logos —
  all local files, embedded or operator-supplied.
- **Export is rendered server-side in Go** (`image`, `image/draw`,
  `golang.org/x/image/draw` for quality scaling, `golang.org/x/image/font` +
  `font/opentype` for text) so exports are deterministic and pixel-accurate
  rather than dependent on browser canvas quirks. **Output: PNG/JPEG** (no
  pure-Go WebP encoder exists; WebP sources can be *decoded*/imported — §10).
- Thumbnails savable as reusable **templates**.
- **Accessibility:** a text-contrast readout to help keep thumbnail titles legible.
- **Import safety:** v1 accepts raster imports (PNG/JPEG, WebP decode) only;
  uploads are decode-verified and size-limited before processing. *Future:* SVG
  import routed through
  [`pbcsvgsanitize`](https://github.com/privatebychoice/pbcsvgsanitize).

### 5.6 Sponsor tracking (deliverables + optional financials)
- **Sponsor** — name, contact, notes, links, brand assets (local files).
- **Campaign** — belongs to a sponsor; name, date window, deliverable spec,
  talking points, promo code, tracking link.
- **Placement** — links a campaign to a `ContentItem`: deliverable checklist,
  per-deliverable status, deadline. Drives the sponsor blurb in the description
  (§5.4) and appears in the pipeline board.
- **Optional financial fields** (nullable, off the critical path): rate,
  currency, invoice status, payment status. No reporting surface in v1 beyond
  simple per-campaign totals.

### 5.7 Media file tracking (catalogue + missing-file detection)
- **Catalogue** by scanning configured media root(s): path, filename, kind
  (video/audio/image/other), size, mtime, and — for supported files —
  container/codec/duration/resolution/fps.
  - Metadata via **`ffprobe`** (JSON) when available; **`abema/go-mp4`** as a
    pure-Go fallback for MP4/MOV when `ffprobe` is absent (metadata only).
  - Assets link to a `ContentItem` and optionally a specific shot; carry a status
    (to-shoot/recorded/imported/edited/used) and notes.
- **Missing-file detection:** store path + size + mtime; a rescan flags moved or
  missing files. **Checksums (SHA-256) are optional and off by default** to avoid
  hashing passes over large media; enabling them adds true integrity + dedupe
  detection.
- *Optional (needs `ffmpeg`):* extract a preview frame for an asset thumbnail.
  Frame decoding cannot be done in pure Go, so this feature is gated on `ffmpeg`
  being installed and degrades gracefully when it is not.

### 5.8 Production board & tasks
- Kanban/board view over `ContentItem.status`; per-item task checklist. This is
  the day-to-day working surface tying the above together.

### 5.9 Blog repurposing & Markdown export (video → blog)
Turn a finished/planned video into a blog post with minimal rework.

- **Repurpose action.** From a video `ContentItem`, *"Repurpose to blog"* creates
  a **new `ContentItem` of type `blog`** in the same channel, with
  `derived_from` set to the source video (provenance link, §3). It is a **one-time
  seed (fork)**, then edited **independently** — later edits to the video's script
  never silently change the blog. A **re-seed** command is available and **warns
  before overwriting** blog edits.
- **Seeding rules** (source → blog draft):
  - **Outline/Layout segments** (§5.2) → section **headings** (H2) with body
    slots, preserving order. Segment notes become editorial hints, not published
    text.
  - **Script** (§5.1) prose → body **paragraphs** under the matching segment; for
    Faceless, the voiceover text becomes the primary prose.
  - **Thumbnail** (§5.5) → **hero image** (front-matter reference).
  - **Media assets** that are images or have extracted preview frames (§5.7) →
    **candidate inline images** the operator can place/keep/drop.
  - **Description** (§5.4) links → a **"Links / further reading"** block.
  - **Channel defaults** seed title, tags, and author.
- **The blog draft is a normal blog `ContentItem`**, so it reuses the existing
  editing surfaces (script/outline editor as the body, media, description blocks).
  No separate blog-only tooling.
- **Markdown export** (applies to **any** blog `ContentItem`, derived or
  natively planned):
  - Emits a **Markdown** file with **YAML front matter**: `title`, `date`,
    `tags`, `hero` (image path), `draft`, and a `source` reference to the origin
    video when derived. Front-matter fields are configurable per channel.
  - **Optional bundled images folder**: copies referenced **local** images next
    to the `.md` and rewrites links to relative paths, producing a portable
    bundle. No image is fetched over the network — local files only.
  - Output path is **configurable** (e.g. an exports dir or the project's `Docs`
    folder, §8.1); nothing is hardcoded. Export is pure text/file assembly — **no
    network, no third-party services**.
  - **Portable by design**: standard Markdown + front matter imports into
    `pbcssg` or any static-site pipeline. `pbccreate` stays **decoupled** from
    `pbcssg` (no shared DB, no format lock-in). *Future:* a pluggable exporter
    interface could add an SSG-specific target without reworking this feature.

---

## 6. UI stack

**Go + local web UI, following the established pattern:** `net/http` (Go 1.22+
`ServeMux` pattern routing — no third-party router), `html/template` (contextual
auto-escaping), and self-hosted **vanilla JS** with no frameworks or CDNs. All
front-end assets (JS, CSS, fonts, templates) are compiled into the binary with
`//go:embed`, so the app is a single self-contained file and every asset is
self-hosted by construction.

The thumbnail editor (§5.5) is the one non-trivial JS surface: a hand-rolled
canvas editor. Its **export** is delegated to Go for a deterministic render.

**Accessibility** is a requirement, not an afterthought: semantic HTML, native
controls over custom widgets, full keyboard operation, visible focus states,
sufficient contrast, and scalable text (WCAG-aligned).

---

## 7. Data model (SQLite)

Schema sketch (columns abbreviated; final DDL lives in migrations). All tables
use integer PKs and `created_at`/`updated_at` timestamps.

- `channels` — name, kind, brand-kit ref, default description-template ref.
- `content_items` — channel_id, type, mode, title, status, target_length,
  `derived_from_id` (nullable self-FK → source item, e.g. blog repurposed from a
  video, §5.9), `last_export_path` (nullable), …
- `scripts` — content_item_id, body, wpm, computed_duration.
- `outline_segments` — content_item_id, position, title, notes, target_seconds,
  scene_notes.
- `shots` — content_item_id, position, description, scene, framing, camera,
  status, notes.
- `thumbnails` — content_item_id, name, canvas_json (layer model), export_path,
  is_template.
- `descriptions` — content_item_id, template_ref, rendered_text.
- `media_assets` — path, kind, size, mtime, duration, width, height, codec, fps,
  sha256 (nullable), status, content_item_id (nullable), shot_id (nullable),
  present (bool), last_seen_at.
- `sponsors` — name, contact, notes.
- `sponsor_campaigns` — sponsor_id, name, starts_on, ends_on, talking_points,
  promo_code, tracking_link, rate (nullable), currency (nullable),
  invoice_status (nullable), payment_status (nullable).
- `sponsor_placements` — campaign_id, content_item_id, deadline.
- `sponsor_deliverables` — placement_id, description, status.
- `tasks` — content_item_id, description, status, position.
- `templates` — mode/channel-scoped defaults (outline, shot-list, description,
  Resolve folder structure).

Config-owned (not in DB): media roots, Resolve folder templates (editable), tool
paths.

---

## 8. DaVinci Resolve integration

**v1: folder scaffolding (all editions) + scripting (Resolve Studio).** Both are
implemented behind a single `ResolveIntegration` interface. Scaffolding needs no
Resolve at all; scripting requires Resolve Studio + Python 3 and is **optional and
runtime-detected** — when unavailable, the app falls back to scaffolding and shows
a clear *"scripting requires DaVinci Resolve Studio + Python 3"* message rather
than failing.

### 8.1 Folder scaffolding (all editions)
- Pure filesystem work: create a project root and a **configurable, per-mode**
  subfolder tree, optionally exporting the script/shot list into a `Docs`
  subfolder. No connection to Resolve; no external dependency.
- Templates are **configurable** (not hardcoded paths — §12). Example default
  tree (illustrative only):

  ```
  <ProjectRoot>/
    01_Footage/        (mode-specific: A-Cam, B-Cam, B-Roll, Screen-Recordings, Cam A/B/C…)
    02_Audio/          (VO, Music, SFX)
    03_Graphics/       (Thumbnails, Lower-Thirds, Overlays)
    04_Project/        (Resolve project files)
    05_Exports/
    06_Assets/         (brand kit, sponsor assets)
    07_Docs/           (exported script, shot list)
  ```
- Path handling is validated to prevent traversal; the operator's configured
  media/project root is the only writable base.

### 8.2 Scripting API (v1 — requires Resolve Studio)
- **Edition constraint:** DaVinci Resolve's **external** scripting API (driving
  Resolve from a separate process) is **Studio-only**. The **free** edition only
  runs scripts from Resolve's built-in Console and cannot be driven by an external
  program — a deliberate Blackmagic edition gate. There is **no pure-Go/native
  route**; the only integration shape is `Go → os/exec → Python 3 helper`
  importing `DaVinciResolveScript` (`scriptapp("Resolve")`), communicating over a
  **JSON stdin/stdout boundary**.
- **v1 capabilities** (the `Scripter` implementation of `ResolveIntegration`):
  - **Create/open** a Resolve project (from the scaffolded `04_Project` context).
  - **Import media into bins/folders** in the media pool, mirroring the scaffolded
    per-mode folder structure (§8.1) and the catalogued assets for the item (§5.7).
  - **Build a timeline from the shot list** (§5.3): create a timeline and append
    the selected clips in shot order; for Multi Cam, group per-camera bins to
    support downstream multicam sync.
- **Detection & degradation:** availability is checked at runtime — required env
  vars present (`RESOLVE_SCRIPT_API`, `RESOLVE_SCRIPT_LIB`, `PYTHONPATH`), a
  usable Python 3, `fusionscript` loadable, and `scriptapp("Resolve")` returning
  non-nil (Resolve must be running). If any check fails, the app uses the
  `Scaffolder` path only and surfaces a clear message; scripting failures never
  corrupt local state (the SQLite plan is the source of truth; Resolve is a sink).
- **Python helper packaging:** the helper script(s) are embedded in the binary via
  `//go:embed` and materialized to the config/cache dir (or piped) at invocation —
  preserving the single-binary distribution. They are invoked with an argument
  slice (never a shell string) and the Resolve env vars, exchange **JSON** on
  stdin/stdout, and contain no secrets. Resolve/Python paths are configurable
  (§2); nothing is hardcoded.
- **Interface boundary:** `ResolveIntegration` exposes both `Scaffolder`
  (filesystem, always available) and `Scripter` (Studio, optional) so callers
  depend on the interface, not the edition.

---

## 9. Privacy & security posture

- **Zero network egress.** `pbccreate` opens no outbound connections: no
  telemetry, analytics, tracking, update checks, external fonts, or CDNs. This is
  a design invariant, not a setting. Any future feature that would contact the
  network must be flagged and opt-in.
- **GPC:** not applicable — there is no public web surface, no third parties, and
  no sale/share of personal data (it is a local single-user tool). Recorded here
  deliberately per standing policy; **if scope ever changes** to a served/public
  or networked surface, GPC obligations must be re-evaluated.
- **Loopback trust boundary.** The server binds `127.0.0.1` only. State-changing
  requests use a CSRF token and same-origin checks; responses set a strict local
  CSP consistent with fully self-hosted assets.
- **Input validation & safe subprocess use.** Uploaded images are decode-verified
  and size-limited; SVG import (Phase 2) goes through `pbcsvgsanitize`. File paths
  are validated against configured roots (no traversal). External tools are
  invoked via `os/exec` with **argument slices, never a shell string** — no shell
  injection; tool paths are validated.
- **Secrets:** none in v1 (plan-only, no OAuth/tokens). Financial fields, if
  used, stay local. Nothing sensitive is logged.
- **Data at rest** lives in local SQLite + media files. The app does not encrypt;
  OS full-disk encryption is the operator's responsibility (documented in the
  README).
- **Logging:** structured `log/slog` at `ERROR/WARN/INFO/DEBUG`; log decision
  points and tool invocations with context, never secrets or personal data.

---

## 10. External dependencies

Vetted for pure-Go, minimal transitive footprint, active maintenance, and
permissive licensing. Full details tracked in
[`external_dependencies.md`](../external_dependencies.md).

**Go modules**

| Module | Version | License | Pure-Go | Purpose |
|---|---|---|---|---|
| `modernc.org/sqlite` | v1.56.0 | BSD-3-Clause | ✅ | SQLite via `database/sql`; CGO-free → single cross-compiled binary. Pulls the cznic `libc`/`mathutil`/`memory` family (pure-Go, one maintainer). Pin major.minor; keep `libc` at the version in the driver's `go.mod`. |
| `golang.org/x/image` | v0.33.0 | BSD-3-Clause | ✅ | Go-team extended stdlib: quality scaling (`draw.CatmullRom`), font/text rendering (`font/opentype`), WebP **decode**. |
| `github.com/abema/go-mp4` *(optional)* | latest | BSD-3-Clause | ✅ | Pure-Go MP4/MOV metadata fallback when `ffprobe` is absent (no frame extraction). |

Everything else is **standard library**: `net/http`, `html/template`,
`encoding/json`, `database/sql`, `embed`, `os/exec`, `log/slog`, `context`,
`crypto/*`, `image`/`image/png`/`image/jpeg`/`image/draw`.

**External system tools** (user-installed; invoked at arm's length via `os/exec`;
**not** in `go.mod` and **never bundled**, so their LGPL/GPL stays on the far side
of the process boundary):

- **`ffprobe`** — media metadata as JSON (primary path).
- **`ffmpeg`** — preview-frame extraction (pure Go cannot decode frames);
  optional, feature degrades gracefully when absent.
- **DaVinci Resolve Studio + Python 3** — for the v1 scripting feature (§8.2).
  Optional and runtime-detected: folder scaffolding and all other features work
  without them; only the scripting operations require them.

Each is detected at startup; absence disables the dependent feature with a clear
message rather than failing.

---

## 11. Build, versioning & release

- **Versioning:** semver git tags starting at `v0.1.0`; releases are deliberate
  (no tag-on-every-push). Cut tags at designated milestones.
- **Build number** (web-UI rule): `1.0.x` where `x` increments before each
  release; surfaced in the UI footer and via `pbccreate version` / a `/version`
  endpoint.
- **Binary:** pure Go, `-trimpath`, statically linked, cross-compiled from one
  machine (enabled by the CGO-free stack). Front-end assets embedded via
  `//go:embed`.
- **Logging:** `log/slog` structured output.

---

## 12. Conventions

- No hardcoded environment-specific values (paths, ports, hostnames): all via
  flags/env with documented defaults; example values in this doc are labeled as
  examples.
- Conventional Commits; commits authored `Private By Choice
  <code@privatebychoice.com>`, no co-author trailer.
- Every external dependency documented in `external_dependencies.md`.

---

## 13. Roadmap (phased)

**Phase 1 — v0.1.0 → v1.0.0 (this spec):**
channels + content items + pipeline board; script, outline/layout, shot list
(mode templates); media catalogue + missing-file detection; thumbnail editor +
Go server-side export; templated descriptions with auto chapters; sponsor
tracking (deliverables + optional financials); **Resolve folder scaffolding +
Studio scripting** (create/open project, import media to bins, build timeline from
shot list) behind the `ResolveIntegration` interface (§8); **blog repurposing
(video → blog) + portable Markdown export** (§5.9).

**Phase 2 (post-v1, later):**
Deeper Resolve automation (color/render-queue, multicam auto-sync);
`pbc-classification` on description/blog links; a pluggable **SSG-specific blog
exporter** (e.g. `pbcssg`-tailored output) alongside the portable Markdown target
(§5.9); SVG thumbnail import via `pbcsvgsanitize`; `ffmpeg` preview-frame
thumbnails; calendar/scheduling view.

**Explicitly out of scope (v1):** publishing/OAuth/platform APIs, network egress
of any kind, multi-user/RBAC, cloud sync, mobile.
