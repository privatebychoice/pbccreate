# External Dependencies — pbccreate

Every external dependency used by `pbccreate`, with version, project URL, and
security/privacy notes. `pbccreate` is local-only and makes no network calls; the
main risk surface is supply-chain (transitive deps) and cross-platform
single-binary buildability — both favor a core-first, no-CGO posture.

Vetting date: 2026-08-08. Versions are pinned to major.minor; the latest stable
patch floats.

---

## Go modules

### `modernc.org/sqlite`
- **Version:** v1.56.0 (embeds SQLite 3.53.x)
- **License:** BSD-3-Clause
- **URL:** https://pkg.go.dev/modernc.org/sqlite (dev: GitLab `cznic/sqlite`)
- **Pure Go:** Yes — SQLite's C amalgamation transpiled to Go; no CGO, no C
  toolchain, no `libsqlite3` on the host. Enables cross-compiling a single static
  binary from one machine.
- **Purpose:** SQLite storage via the standard `database/sql` interface (driver
  name `"sqlite"`).
- **Transitive deps:** the cznic runtime family — `modernc.org/libc`,
  `modernc.org/mathutil`, `modernc.org/memory` (+ small helpers). All pure-Go,
  same maintainer, BSD-3-Clause. Effectively one dependency *family* from a
  single source rather than a broad graph.
- **Security notes:** no notable CVEs against the driver. Because upstream SQLite
  C is *transpiled*, SQLite security fixes land only when modernc re-generates and
  tags a release — pin major.minor and let patches float. **Keep
  `modernc.org/libc` at the exact version in the driver's own `go.mod`** (let
  `go mod tidy` manage it; do not hand-bump `libc`).
- **Why not `mattn/go-sqlite3`:** requires CGO → C toolchain + no clean
  cross-compilation; rejected for the single-binary goal.

### `golang.org/x/image`
- **Version:** v0.33.0
- **License:** BSD-3-Clause
- **URL:** https://pkg.go.dev/golang.org/x/image
- **Pure Go:** Yes. Maintained by the Go team as an official subrepository (same
  governance/review as the standard library) — treated as trusted extended stdlib.
- **Purpose:** thumbnail export — quality scaling (`draw.CatmullRom`,
  `draw.BiLinear`), text rendering (`font`, `font/opentype`, `font/sfnt`;
  `font/gofont` for a self-hosted default face), and WebP **decode**.
- **Transitive deps:** other `golang.org/x/*` modules only (e.g.
  `golang.org/x/text`) — Go-team, BSD-3-Clause. No third-party graph.
- **Security notes:** no notable concerns. **No WebP *encoder*** exists in
  pure Go — thumbnail export stays PNG/JPEG (WebP sources can be decoded/imported).

### `github.com/abema/go-mp4` *(optional)*
- **Version:** latest (evaluate at adoption)
- **License:** BSD-3-Clause
- **URL:** https://github.com/abema/go-mp4
- **Pure Go:** Yes.
- **Purpose:** pure-Go fallback for **MP4/MOV metadata** (duration, dimensions,
  codec fourCC) when `ffprobe` is not installed. Reads the ISO-BMFF box structure;
  **cannot decode frames** (no thumbnail extraction) and covers MP4/MOV only.
- **Security notes:** actively maintained (ABEMA streaming stack). Parses
  untrusted file structure — treat inputs defensively (size limits, error
  handling). Add only if the no-ffprobe fallback is wanted.

---

## External system tools (NOT Go modules)

User-installed; invoked at arm's length via `os/exec` (argument slices, never a
shell string). **Not** in `go.mod` and **never bundled** — so their LGPL/GPL
licensing stays firmly on the far side of the process boundary and imposes no
obligation on `pbccreate`. Each is detected at startup; absence disables the
dependent feature with a clear message.

### `ffprobe`
- **Purpose:** media metadata (duration/resolution/codec/fps) as JSON —
  `-v quiet -print_format json -show_format -show_streams`.
- **License:** part of FFmpeg (LGPL-2.1+, or GPL if built with GPL components).
  Arm's-length subprocess invocation is not linking → no copyleft obligation on
  `pbccreate`.
- **Distribution:** require the operator to install (Homebrew/apt); do not bundle.

### `ffmpeg`
- **Purpose:** extract a preview frame for an asset/thumbnail (pure Go cannot
  decode frames). Optional; the feature degrades gracefully when absent.
- **License / distribution:** same as `ffprobe` above.

### DaVinci Resolve Studio + Python 3 *(v1 — optional, runtime-detected)*
- **Purpose:** external scripting — create/open project, import media to bins,
  build a timeline from the shot list. See SPEC §8.2.
- **Edition constraint:** external scripting is **Studio-only**; the free Resolve
  edition cannot be driven by an external process. No pure-Go/native route — the
  only path is `Go → os/exec → Python 3 helper` importing `DaVinciResolveScript`,
  over a JSON boundary. Requires `RESOLVE_SCRIPT_API` / `RESOLVE_SCRIPT_LIB` /
  `PYTHONPATH` and a running Resolve instance.
- **Status:** in v1. Optional + runtime-detected — folder scaffolding and every
  other feature work without it; only the scripting operations require Studio +
  Python 3. The Python helper is embedded via `//go:embed`, invoked with an
  argument slice (never a shell string), and exchanges JSON on stdin/stdout.
- **Python version:** target CPython 3.x compatible with the installed Resolve
  build's `fusionscript` ABI; the interpreter path is configurable
  (`PBCCREATE_PYTHON`).
