package resolve

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSanitizeProjectName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Best VPN 2026", "Best VPN 2026"},
		{"a/b/c", "a-b-c"},
		{"../evil", "evil"},
		{"..", ""},
		{"/", ""},
		{"  spaced  ", "spaced"},
		{"weird***name", "weird-name"},
		{"a///b", "a-b"},
		{"", ""},
	}
	for _, c := range cases {
		if got := SanitizeProjectName(c.in); got != c.want {
			t.Errorf("SanitizeProjectName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultTemplateFootageByMode(t *testing.T) {
	cases := map[string][]string{
		"single_cam": {"A-Cam", "B-Roll"},
		"multi_cam":  {"Cam-A", "Cam-B", "Cam-C", "B-Roll"},
		"obs":        {"Screen-Recordings", "Webcam", "B-Roll"},
		"faceless":   {"Screen-Recordings", "Stock", "B-Roll"},
		"":           {"A-Cam", "B-Roll"}, // default/blank mode
	}
	for mode, want := range cases {
		tmpl := DefaultTemplate(mode)
		if len(tmpl.Folders) == 0 || tmpl.Folders[0].Name != "01_Footage" {
			t.Fatalf("mode %q: first folder is not 01_Footage: %+v", mode, tmpl.Folders)
		}
		var got []string
		for _, f := range tmpl.Folders[0].Children {
			got = append(got, f.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("mode %q footage = %v, want %v", mode, got, want)
		}
	}
}

func TestScaffoldCreatesTreeAndDocs(t *testing.T) {
	base := t.TempDir()
	spec := ScaffoldSpec{
		Base:        base,
		ProjectName: "Best VPN 2026",
		Mode:        "multi_cam",
		Template:    DefaultTemplate("multi_cam"),
		Docs: map[string]string{
			"script.md":   "# hello\n",
			"shotlist.md": "1. wide\n",
		},
	}

	res, err := FSScaffolder{}.Scaffold(spec)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	wantRoot := filepath.Join(base, "Best VPN 2026")
	if res.ProjectRoot != wantRoot {
		t.Errorf("ProjectRoot = %q, want %q", res.ProjectRoot, wantRoot)
	}

	// A sampling of the tree, including a mode-specific footage subfolder.
	for _, rel := range []string{
		"01_Footage", filepath.Join("01_Footage", "Cam-B"),
		filepath.Join("02_Audio", "Music"), "04_Project", DocsFolder,
	} {
		if fi, err := os.Stat(filepath.Join(wantRoot, rel)); err != nil || !fi.IsDir() {
			t.Errorf("expected dir %q: err=%v", rel, err)
		}
	}

	// Docs written into the Docs folder, reported deterministically (sorted).
	for _, rel := range []string{
		filepath.Join(DocsFolder, "script.md"),
		filepath.Join(DocsFolder, "shotlist.md"),
	} {
		if _, err := os.Stat(filepath.Join(wantRoot, rel)); err != nil {
			t.Errorf("expected doc %q: %v", rel, err)
		}
	}
	if !slices.Contains(res.Docs, filepath.Join(DocsFolder, "script.md")) {
		t.Errorf("Result.Docs missing script.md: %v", res.Docs)
	}
	if b, _ := os.ReadFile(filepath.Join(wantRoot, DocsFolder, "script.md")); string(b) != "# hello\n" {
		t.Errorf("script.md contents = %q", b)
	}

	// Every reported dir must live under the project root (no escape).
	for _, d := range res.Dirs {
		if filepath.IsAbs(d) {
			t.Errorf("dir %q is absolute, expected project-relative", d)
		}
	}

	// Idempotent: a second run over the same base succeeds.
	if _, err := (FSScaffolder{}).Scaffold(spec); err != nil {
		t.Errorf("second Scaffold: %v", err)
	}
}

func TestScaffoldConfinesToBase(t *testing.T) {
	base := t.TempDir()

	// A traversal-style name is sanitized to a safe segment and stays inside base.
	res, err := FSScaffolder{}.Scaffold(ScaffoldSpec{
		Base:        base,
		ProjectName: "../../etc",
		Template:    DefaultTemplate(""),
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	rel, err := filepath.Rel(base, res.ProjectRoot)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || rel == "." {
		t.Fatalf("project root %q escaped base %q (rel=%q)", res.ProjectRoot, base, rel)
	}

	// A name that sanitizes to nothing is rejected.
	if _, err := (FSScaffolder{}).Scaffold(ScaffoldSpec{Base: base, ProjectName: ".."}); err == nil {
		t.Error("expected error for empty-after-sanitization name")
	}

	// An empty base is rejected.
	if _, err := (FSScaffolder{}).Scaffold(ScaffoldSpec{Base: "", ProjectName: "x"}); err == nil {
		t.Error("expected error for empty base")
	}
}

func TestDetectScripting(t *testing.T) {
	found := func(string) (string, error) { return "/usr/bin/python3", nil }
	notFound := func(string) (string, error) { return "", errors.New("not found") }

	full := map[string]string{
		"RESOLVE_SCRIPT_API": "/api",
		"RESOLVE_SCRIPT_LIB": "/lib.so",
		"PYTHONPATH":         "/modules",
	}
	getenv := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	// All prerequisites present.
	if st := DetectScripting(getenv(full), "python3", found); !st.Available || len(st.Missing) != 0 {
		t.Errorf("full env: Available=%v Missing=%v", st.Available, st.Missing)
	}

	// Missing one env var.
	partial := map[string]string{"RESOLVE_SCRIPT_API": "/api", "PYTHONPATH": "/m"}
	st := DetectScripting(getenv(partial), "python3", found)
	if st.Available {
		t.Error("expected unavailable with a missing env var")
	}
	if !slices.Contains(st.Missing, "RESOLVE_SCRIPT_LIB") {
		t.Errorf("Missing should list RESOLVE_SCRIPT_LIB: %v", st.Missing)
	}

	// Python not on PATH.
	st = DetectScripting(getenv(full), "python3", notFound)
	if st.Available {
		t.Error("expected unavailable when python3 is not found")
	}
	if len(st.Missing) != 1 {
		t.Errorf("expected exactly the python miss, got %v", st.Missing)
	}
}
