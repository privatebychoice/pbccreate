package media

import (
	"path/filepath"
	"testing"
)

func TestKind(t *testing.T) {
	cases := map[string]string{
		"/a/b/clip.MP4":   "video",
		"/a/song.flac":    "audio",
		"/a/thumb.PNG":    "image",
		"/a/notes.txt":    "other",
		"/a/no-extension": "other",
	}
	for path, want := range cases {
		if got := Kind(path); got != want {
			t.Errorf("Kind(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestWithinRoots(t *testing.T) {
	root := filepath.FromSlash("/media/footage")
	roots := []string{root}

	within := []string{
		filepath.Join(root, "a.mp4"),
		filepath.Join(root, "sub", "b.mov"),
		filepath.Join(root, "..hidden.mp4"), // ".." in a filename does not escape
	}
	for _, p := range within {
		if !WithinRoots(p, roots) {
			t.Errorf("WithinRoots(%q) = false, want true", p)
		}
	}

	outside := []string{
		filepath.FromSlash("/media/other/a.mp4"),
		filepath.Join(root, "..", "escape.mp4"),
		"relative/path.mp4", // not absolute
	}
	for _, p := range outside {
		if WithinRoots(p, roots) {
			t.Errorf("WithinRoots(%q) = true, want false", p)
		}
	}

	// No roots configured: any absolute path is accepted, relatives rejected.
	if !WithinRoots(filepath.FromSlash("/anywhere/x.mp4"), nil) {
		t.Error("with no roots, absolute path should be accepted")
	}
	if WithinRoots("rel.mp4", nil) {
		t.Error("relative path should be rejected even with no roots")
	}
}
