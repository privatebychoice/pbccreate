package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDir(t *testing.T) {
	root := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("clip.mp4")
	write("photo.JPG")
	write("notes.txt")          // "other" -> skipped
	write("sub/voiceover.flac") // nested media -> included
	write("sub/.hidden-notes")  // no media ext -> skipped

	files, err := ScanDir(root)
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("found %d media files, want 3: %+v", len(files), files)
	}

	// Sorted by path; verify kinds.
	byKind := map[string]string{}
	for _, f := range files {
		byKind[filepath.Base(f.Path)] = f.Kind
	}
	if byKind["clip.mp4"] != "video" || byKind["photo.JPG"] != "image" || byKind["voiceover.flac"] != "audio" {
		t.Errorf("unexpected kinds: %v", byKind)
	}
	for _, f := range files {
		if f.Size == 0 {
			t.Errorf("size not populated for %s", f.Path)
		}
	}
}

func TestScanDirEmpty(t *testing.T) {
	files, err := ScanDir(t.TempDir())
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("want 0 files, got %d", len(files))
	}
}
