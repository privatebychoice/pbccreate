// Package buildinfo exposes version and build-number metadata for pbccreate.
package buildinfo

// Version is the semantic version. Overridden at release via -ldflags; "dev"
// during local development (see docs/SPEC.md §11).
var Version = "dev"

// Build is the web-UI build number — the third component of 1.0.x — surfaced in
// the UI footer and by `pbccreate version` (see docs/SPEC.md §11).
var Build = "0"

// String returns a human-readable version string, e.g. "0.1.0 (build 3)".
func String() string {
	return Version + " (build " + Build + ")"
}
