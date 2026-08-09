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
- **Repurpose video → blog** (and **→ short-form clips**): derive editable
  content from a video's script/outline/media — portable Markdown + front matter
  for blogs, timestamped clip plans for Shorts/Reels.
- **Organize the whole operation**: an idea backlog with scoring, content pillars,
  series/playlist planning, reusable formats, internal project labels, a
  cross-project asset library, and gear/location profiles.
- **Learn and improve**: retrospectives, an experiments log, and **local
  (no-egress) personal analytics** on your own cadence and throughput.
- **Track releases and learn from them**: record the published output (file name,
  platform video ID, URL, posted date, visibility), an SEO **tag library**,
  required **attributions/licensing credits**, linked sponsor requirements, and
  post-publish **improvement notes** — so each video is organized and the next one
  is better.
- Stay **core-first**: standard library for nearly everything; a tiny, vetted,
  pure-Go dependency set (§10); external media/Resolve tooling invoked only at
  arm's length as separate processes.

**Non-goals (v1)**

- **No network egress in v1.** No publishing, no OAuth, no platform APIs, no
  telemetry, no analytics, no update checks, no external fonts/CDNs. Content is
  drafted and exported for the operator to post manually. (A **YouTube API**
  integration is a **post-v1** goal — opt-in and off by default, gated by the
  default-deny egress override, §9.1 and §13.)
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
| Asset-library root | `PBCCREATE_ASSET_ROOT` | *(unset — optional; §5.16)* |
| Listen address | `PBCCREATE_ADDR` | `127.0.0.1:8787` |
| `ffprobe` path | `PBCCREATE_FFPROBE` | resolved from `PATH` |
| `ffmpeg` path | `PBCCREATE_FFMPEG` | resolved from `PATH` |
| `python3` path (Resolve helper) | `PBCCREATE_PYTHON` | resolved from `PATH` |
| Network egress override | `PBCCREATE_NETWORK` / config | **disabled** (default-deny; §9.1) |

*(Default paths above are examples; nothing environment-specific is compiled in.)*

**Storage:** a single SQLite database via `database/sql` + `modernc.org/sqlite`
(pure Go, §10). WAL mode, `busy_timeout` set, single-writer discipline
(`SetMaxOpenConns(1)` or serialized writes) — appropriate for one local user.

---

## 3. Domain model

```
Channel (brand, e.g. a YouTube channel / blog)
└── ContentItem (video | short | blog | social)   ← the central unit
    ├── derived_from → ContentItem (nullable; blog from video §5.9, or short/clip §5.17)
    ├── mode: faceless | single_cam | multi_cam | obs   (videos)
    ├── status: idea → scripting → shooting → editing → scheduled → published → archived
    ├── Script            (beats/sections; VO for faceless)
    ├── Outline/Layout    (ordered segments + timing; scene composition for OBS/multicam)
    ├── ShotList          (ordered shots; columns vary by mode)
    ├── Thumbnail(s)      (layered designs → exported PNG/JPG)
    ├── Description       (templated blocks; chapters, links, sponsor blurb, disclosure)
    ├── MediaAsset(s)     (catalogued files linked to item/shot)
    ├── SponsorPlacement(s)  (deliverables from a campaign, tied to this item)
    ├── Tag(s)            (channel tag library; feeds YouTube tags + hashtags, §5.10)
    ├── Attribution(s)    (music/stock/font credits + licenses, §5.11)
    ├── LicenseFile(s)    (uploaded legal docs/certificates on disk, §5.11)
    ├── Publication(s)    (per-platform: output file, video ID, URL, posted date, §5.12)
    ├── Retrospective     (what worked / improvement notes, §5.12)
    └── Task(s)           (production checklist)

Sponsor ─< SponsorCampaign ─< SponsorPlacement >─ ContentItem
```

- **Channel** — a brand/destination (the operator may run several, e.g. distinct
  YouTube channels/blogs). Every `ContentItem` belongs to one channel; channels
  carry default templates (description blocks, brand kit) inherited by their
  items.
- **ContentItem** — the unit of planning. Carries type, creator mode (videos),
  and a **pipeline status** used for a kanban/board view.

**Supporting entities** (channel- or globally-scoped; detailed in §5.13–§5.19):
`Idea` (topics log), `Pillar`, `ProjectLabel`, `Series`/playlist, `Format`,
`TitleCandidate`, `HookBankEntry`, `InspirationItem`, `GearProfile`,
`LocationProfile`, `Take`, `ChecklistTemplate`/`ChecklistRun`, `AssetLibraryItem`
(cross-project), `MusicCue`, `CaptionTrack`, `DeliveryPreset`, `SponsorDeal`,
`MediaKit`, `Experiment`, `TimeEntry`.

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
  attached campaign, §5.6), **hashtags** (from the item's tags, §5.10), a
  **credits block** (auto-assembled from attributions marked for inclusion,
  §5.11), and a **disclosure/affiliate** block.
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

### 5.10 Tags & keyword library (SEO)
- A **channel-scoped tag library** the operator builds up and reuses, so keywords
  stay consistent across videos instead of being retyped per upload.
- Each `ContentItem` has an assigned tag set (many-to-many with the library).
  Assigned tags feed both the **YouTube tags** field the operator pastes at upload
  time and, where appropriate, the **hashtags** block of the description (§5.4).
- Utilities: reuse-count/frequency per tag (spot your workhorse keywords),
  per-item copy-to-clipboard of the tag list, and a soft length hint (YouTube's
  ~500-char tag budget). No network — this is organization/recall, not analytics.

### 5.11 Attributions & licensing credits
Remember exactly what third-party assets a video used and what crediting each
requires — the difference between a clean upload and a takedown/claim.

- Per `ContentItem`, an **attribution list**: asset name, **kind** (music / SFX /
  stock footage / image / font / other), **provider/source**, **license** (e.g.
  CC-BY, royalty-free, licensed), an optional **license/reference ID** (e.g. a
  subscription license number), the **required credit text**, a source URL, and
  an optional link to the catalogued `MediaAsset` (§5.7) it corresponds to.
- **`included_in_description`** flag per attribution; the description (§5.4) can
  **auto-assemble a credits block** from attributions marked for inclusion, so
  required credits actually make it into the upload.
- Surfaced in the pre-publish checklist (§5.12): required attributions that are
  not yet included are flagged before the item can be marked published.
- **License files (per project).** Some licenses ship as a document — a per-video
  license certificate (PDF), a purchase receipt, or plain-text terms — that must
  be *kept*, distinct from the credit text pasted into the description. Each
  content item has a **`Licenses` folder** under the app data dir
  (`<data_dir>/licenses/<content_item_id>/`) and an **upload** control that copies
  any number of files into it. Per file we track: original filename, a
  description, **what assets the license applies to**, an optional link to the
  **external asset provider** (§5.20), size, and upload date. Uploads are
  **stored opaquely and served download-only** (never rendered inline / executed;
  see §9). This **complements, not replaces**, the description attributions above:
  attributions are the credit text that ships in the upload; license files are the
  legal record on disk. An optional `provider_id` link is also added to
  attributions so a credit can name a registered provider while keeping its
  free-text `provider` for one-offs.

### 5.12 Publication record, retrospective & pre-publish checklist
- **Publication record — per platform** (a `ContentItem` may be posted to more
  than one; YouTube is the primary case): **platform**, **published title**,
  **external video ID** (e.g. the YouTube video ID), **URL**, **output file name**
  (the rendered master — a path or a link to a `MediaAsset`), **posted date**,
  **visibility** (public/unlisted/private/scheduled), an optional **tag snapshot**
  (the exact tags used, if they differed from the item's set), and **notes**.
- **Retrospective (improvement notes):** per `ContentItem`, structured
  reflection — **what worked**, **what to improve next time**, a reviewed-on date,
  and optional **manual performance notes**. *In v1, metrics are entered by hand:*
  `pbccreate` makes no network calls, so it does **not** fetch analytics from any
  platform. Auto-filling publication records and pulling analytics via the
  **YouTube API** is a **post-v1** goal (opt-in, off by default, gated by the
  default-deny egress override — §9.1, §13).
- **Pre-publish checklist:** a per-item readiness view that aggregates the pieces
  so nothing ships half-done — required **sponsor deliverables** complete (§5.6),
  required **attributions** present and marked included (§5.11), **thumbnail**
  exported (§5.5), **description** rendered with tags (§5.4/§5.10), and an
  **output file** recorded. The operator marks the item **published** when the
  checklist passes, or explicitly overrides with a reason.

### 5.13 Ideation & strategy
- **Topics / idea log** *(v1)* — capture loose ideas fast (title, note, source,
  project labels). Ideas are lightweight pre-`ContentItem` records; **promote** an
  idea to a `ContentItem` (entering the pipeline at status `idea`).
- **Idea scoring** *(v1)* — optional ICE/RICE fields (impact, confidence, effort)
  → a computed priority that ranks the backlog instead of leaving it flat.
- **Content pillars** *(v1)* — define the channel's core pillars/themes, assign
  items to them, and view **coverage balance** over time (spot a neglected pillar).
- **Working titles & A/B candidates** *(v1)* — several candidate titles per item,
  mark the chosen one; a channel **title swipe file** of patterns that worked.
- **Hook / intro bank** *(Phase 2)* — reusable, taggable library of openers
  (the first 15s), insertable into a script.
- **Inspiration / swipe board** *(Phase 2)* — save **local** reference images /
  screenshots + notes, tagged. No network scraping.
- **Keyword volume / trend lookup** *(Phase 2, egress)* — automated search-volume/
  trend data via the default-deny egress override (§9.1); manual keyword notes
  stay local in v1.

### 5.14 Planning structures
- **Series / playlist planner** *(v1)* — group items into series/playlists, order
  episodes, per-playlist description, track done vs planned, and **continuity/arc
  notes** between episodes. Maps to a YouTube playlist at publish.
- **Recurring formats** *(v1)* — named, reusable "formats" (e.g. tutorial, review,
  essay), each bundling a default outline, shot-list, thumbnail template, and
  checklist. A layer *above* creator modes (§4); selecting a format seeds a new
  item.
- **Project labels** *(v1)* — internal organizational labels (e.g. `evergreen`,
  `needs-reshoot`, `priority`), a **separate namespace** from the outward-facing
  SEO tag library (§5.10). Filter/search the board by label.

### 5.15 Production toolkit
- **Gear & settings log** *(v1)* — record camera/lens/mic/lighting/settings per
  shoot; reusable **gear profiles** to recall your best setup per format/mode.
- **Location / set profiles** *(v1)* — reusable locations with notes (power,
  noise, golden-hour, permissions).
- **Take / shot tracking** *(v1)* — mark good/circle takes and per-take notes
  against shot-list rows and their media (§5.3/§5.7).
- **Per-stage checklists / SOPs** *(v1)* — reusable checklists per stage
  (pre-shoot, shoot-day, edit, publish). The pre-publish checklist (§5.12) becomes
  one instance of this general mechanism.
- **Teleprompter mode** *(Phase 2)* — scroll the script in-browser for single-cam
  reads (speed/size controls). Local only.

### 5.16 Asset library & delivery
- **Cross-project asset library** *(v1)* — a persistent, tagged bank of reusable
  **owned** assets (B-roll, music, SFX, graphics, brand elements) findable across
  all projects. Extends per-project media tracking (§5.7) and links to
  attributions/licensing (§5.11).
- **Music cue sheet** *(v1)* — per-item track list with in/out points + license,
  feeding attributions (§5.11); useful for Content ID disputes.
- **Caption / subtitle tracking** *(Phase 2)* — status + an SRT reference (or
  draft caption text) per item.
- **Export / delivery presets** *(Phase 2)* — per-platform delivery specs
  (resolution, aspect, codec, length caps) as a reusable checklist.

### 5.17 Repurposing — short-form
- **Shorts / clips planner** *(Phase 2)* — derive short-form clips
  (Shorts/Reels/TikTok) from a long video using the same **derived-`ContentItem`**
  pattern as blog repurposing (§5.9): source timestamps, hook, caption, target
  aspect. `derived_from` links back to the source video. Adds `short` to the
  content-item types.

### 5.18 Sponsorship & business
- **Sponsor CRM** *(Phase 2)* — lift sponsors from per-video placements (§5.6) to
  a relationship view: outreach status, deals in negotiation, rate card, past
  deals, renewal reminders, and local file refs (briefs, contracts, brand
  guidelines).
- **Media-kit data** *(Phase 2)* — store your own stats/pricing to assemble a
  media kit (export as Markdown/doc; no auto-fetch).
- **Revenue dashboard** *(Phase 2)* — aggregate the optional sponsor financials
  (§5.6) into simple totals/trends.

### 5.19 Learning, insights & workflow
- **Global search** *(v1)* — full-text search across scripts, ideas, notes, and
  metadata.
- **Data export, backup & bulk import** *(v1)* — full DB export/backup and bulk
  import (e.g. backfill already-published videos). Reinforces data portability and
  no lock-in.
- **Experiments log** *(Phase 2)* — structured record of deliberate experiments
  (thumbnail style, hook, length, upload time) and outcomes, cross-linked to
  items/pillars/tags. Turns retrospectives (§5.12) into repeatable learning.
- **Personal analytics (no egress)** *(Phase 2)* — insights from **local** data
  only: output cadence, backlog health, time-to-publish, per-stage effort. No
  external calls.
- **Time tracking** *(Phase 2)* — optional per-item/per-stage timers to improve
  future estimates; feeds personal analytics.

### 5.20 External asset providers & subscriptions
Track the 3rd-party media services you subscribe to (e.g. music/SFX/stock/footage
libraries) so licenses and attributions can point at a known provider instead of
being retyped per video.

- **Provider registry** *(v1)* — a **dedicated, operator-wide** (global, not
  channel-scoped) section: one row per service. Fields: **name**, **service type**
  (music / sfx / stock / images / fonts / other), **website URL**, **plan/tier**,
  **billing cycle**, **renewal date**, **status** (active / lapsed), and **terms
  notes**. A **portal/account URL only** may be stored — **never** logins,
  passwords, API keys, or any credential (plan-only, no secrets; §9).
- **Used as a selector** — the provider list drives the *external asset provider*
  dropdown when uploading a per-project **license file** (§5.11) and the optional
  provider link on an **attribution** (§5.11). Free-text entry remains available
  for one-off providers with no subscription.
- **Renewal awareness** *(v1)* — surface upcoming/lapsed renewals so a license
  does not silently expire mid-project. No egress: this is local recall, not
  billing integration.
- Relationship to the cross-project **asset library** (§5.16): the asset library
  catalogues the *assets you own/use*; this registry catalogues the *services and
  their license terms* those assets came from.

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
- `tags` — channel_id, name (unique per channel). `content_item_tags` —
  content_item_id, tag_id (many-to-many, §5.10).
- `attributions` — content_item_id, kind, name, provider, provider_id (nullable
  FK → asset_providers, §5.20), license, license_ref (nullable), credit_text, url,
  media_asset_id (nullable), included_in_description (bool) (§5.11).
- `license_files` — content_item_id, provider_id (nullable FK → asset_providers),
  original_filename, stored_name, description, applies_to, size_bytes, uploaded_at.
  Files live at `<data_dir>/licenses/<content_item_id>/`; served download-only
  (§5.11, §9).
- `publications` — content_item_id, platform, published_title, external_id (e.g.
  YouTube video ID), url, output_file (path or media_asset_id), posted_at,
  visibility, tags_snapshot (nullable), notes (§5.12).
- `retrospectives` — content_item_id, what_worked, to_improve, performance_notes
  (manual), reviewed_at (§5.12).
- `templates` — mode/channel-scoped defaults (outline, shot-list, description,
  Resolve folder structure).

Release/organization (§5.13–§5.19):

- `ideas` — channel_id, title, note, source, ice_impact, ice_confidence,
  ice_effort, score (computed), status, promoted_content_item_id (nullable) (§5.13).
- `pillars` — channel_id, name, description. `content_item_pillars` — join (§5.13).
- `project_labels` — channel_id, name, color. `content_item_labels` — join
  (organizational; **distinct** from `tags`/`content_item_tags`, §5.14/§5.10).
- `series` — channel_id, name, description. `series_items` — series_id,
  content_item_id, position, arc_notes (§5.14).
- `formats` — channel_id, name, outline/shot-list/thumbnail-template/checklist
  refs (§5.14).
- `title_candidates` — content_item_id, text, chosen (bool). `title_swipe` —
  channel_id, pattern, note (§5.13).
- `hook_bank` — channel_id, text, tags, note (§5.13).
- `inspiration_items` — channel_id, kind, path/note, tags (§5.13).
- `gear_profiles` — channel_id, name, details. `content_item_gear` — item↔gear
  used (§5.15).
- `location_profiles` — channel_id, name, notes (§5.15).
- `takes` — shot_id, media_asset_id (nullable), rating/circle (bool), notes (§5.15).
- `checklist_templates` — scope, stage, items. `checklist_runs` — content_item_id,
  template_id, per-item state (the pre-publish checklist is one run) (§5.15/§5.12).
- `asset_providers` — name, service_type, website_url, plan_tier, billing_cycle,
  renewal_on (nullable), status, terms_notes, portal_url (nullable; **no
  credentials**). Operator-wide (not channel-scoped); drives the provider selector
  for license files and attributions (§5.20).
- `asset_library` — scope, kind, path, tags, license, attribution_ref
  (cross-project owned assets, §5.16).
- `music_cues` — content_item_id, asset_ref, in_point, out_point, license (§5.16).
- `caption_tracks` — content_item_id, status, srt_path/draft (§5.16).
- `delivery_presets` — channel_id, platform, resolution, aspect, codec, length_cap
  (§5.16).
- `sponsor_deals` — sponsor_id, stage/status, rate, dates, renewal_at.
  `sponsor_files` — sponsor_id, path/label. `media_kit` — channel_id, stats,
  pricing (§5.18).
- `experiments` — content_item_id (nullable), hypothesis, variable, outcome; links
  to pillars/tags (§5.19).
- `time_entries` — content_item_id, stage, seconds, started_at (§5.19).
- `clip_sources` — content_item_id (the short), source_item_id, in_point,
  out_point (§5.17; the short itself is a `content_items` row with type `short`
  and `derived_from_id`).

Config-owned (not in DB): media roots, **asset-library root**, Resolve folder
templates (editable), tool paths, network egress policy (§9.1).

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

- **Network egress is default-deny (§9.1).** In v1 `pbccreate` opens no outbound
  connections at all — no telemetry, analytics, tracking, update checks, external
  fonts, or CDNs. Egress is not a permanent wall but a **default-off, operator-
  controlled capability**: with the override off — the default, and the entirety of
  v1 — the app is genuinely egress-free. The layered policy governing it is §9.1.
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
- **License-file uploads (§5.11).** Legal documents (PDF/PNG/JPG/WEBP/TXT) are
  **not** parsed or decode-verified as images; they are stored **opaquely** under
  `<data_dir>/licenses/<content_item_id>/` with an app-generated `stored_name`
  (the original filename is metadata only — never used as a path), size-capped,
  and extension-allowlisted. They are served **download-only**: `Content-Disposition:
  attachment`, `X-Content-Type-Options: nosniff`, and a generic content type, so an
  uploaded `.html`/`.svg` can never render or execute in the browser. No provider
  credentials are ever stored (§5.20) — portal URLs only.
- **Secrets:** none in v1 (plan-only, no OAuth/tokens). Financial fields, if
  used, stay local. Nothing sensitive is logged.
- **Data at rest** lives in local SQLite + media files. The app does not encrypt;
  OS full-disk encryption is the operator's responsibility (documented in the
  README).
- **Logging:** structured `log/slog` at `ERROR/WARN/INFO/DEBUG`; log decision
  points and tool invocations with context, never secrets or personal data.

### 9.1 Network egress policy (default-deny override)
Egress is a capability the operator explicitly enables — not a wall, and not
always-on. It is enforced by a **single choke-point HTTP client** (stdlib
`net/http` with a custom transport/dialer) that all networked code must route
through, so no individual feature can bypass the policy. The mechanism is
**specified now and implemented alongside the first networked feature** (the
post-v1 YouTube API, §13); v1 contains no networked code paths and makes no
connections.

Layers, all **default-deny**:

1. **Master switch** (`network.enabled`, default **off**). While off, the
   choke-point client refuses every dial and all networked features are hard
   disabled regardless of their own settings — a true kill-switch.
2. **Per-integration opt-in** (e.g. `youtube.enabled`, default **off**). A feature
   may connect only when the master switch **and** its own toggle are on.
3. **Destination allowlist.** Outbound requests are limited to an explicit list of
   hosts/endpoints; each integration declares what it needs and nothing outside
   the allowlist is permitted. Enforced at the choke-point.
4. **Egress audit log.** Every outbound request is recorded — timestamp,
   integration, method, host/endpoint, purpose, and a size/summary — **never
   secrets or tokens** — giving the operator a verifiable record (satisfies the
   "identify every external request" standard).
5. **Visible indicator.** The UI shows a persistent indicator whenever egress is
   enabled and which integrations are active, so the operator always knows the app
   can reach the network.

**Credentials.** Any OAuth/API credentials (e.g. the operator's own Google OAuth
for YouTube) are stored locally via the OS secret store or a secrets file — never
committed, never logged, never in client-side code. **Configuration** for all of
the above lives in config/env (§2); nothing is hardcoded. GPC remains N/A (no
served/public surface); if a future integration ever *discloses personal data to a
third party*, that must be flagged per the project's privacy standards before it
ships.

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
(video → blog) + portable Markdown export** (§5.9); **release tracking** — SEO tag
library, attributions/licensing, per-platform publication records, retrospectives,
and a pre-publish checklist (§5.10–§5.12); **organization core** — idea log +
scoring, content pillars, series/playlist planner, recurring formats, project
labels (§5.13–§5.14); **production toolkit** — gear/location profiles, take
tracking, per-stage checklists/SOPs (§5.15); **cross-project asset library** +
music cue sheet (§5.16); working titles/swipe file (§5.13); **global search** +
data export/backup/bulk import (§5.19).

**Phase 2 (post-v1, later):**
**YouTube API integration** (opt-in, off by default) — auto-fill publication
records (video ID, URL, posted date, visibility) and pull analytics into
retrospectives (§5.12); uses the operator's own Google OAuth; every request
identified/documented (§9.1). This is the first feature to use the **default-deny
egress override** (§9.1), and it ships that egress guard (master switch,
per-integration opt-in, allowlist, audit log, UI indicator) alongside it. Deeper
Resolve automation (color/render-queue, multicam auto-sync);
`pbc-classification` on description/blog links; a pluggable **SSG-specific blog
exporter** (e.g. `pbcssg`-tailored output) alongside the portable Markdown target
(§5.9); SVG thumbnail import via `pbcsvgsanitize`; `ffmpeg` preview-frame
thumbnails; calendar/scheduling view; **short-form clips planner** (§5.17);
**hook/intro bank**, **inspiration board**, and **keyword lookup** (egress) (§5.13);
**teleprompter mode**, **caption tracking**, **delivery presets** (§5.15–§5.16);
**sponsor CRM**, **media-kit data**, **revenue dashboard** (§5.18); **experiments
log**, **personal (no-egress) analytics**, **time tracking** (§5.19).

**Explicitly out of scope (v1):** publishing/OAuth/platform APIs, network egress
of any kind, multi-user/RBAC, cloud sync, mobile.
