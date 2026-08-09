package resolve

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Folder is a node in a scaffold template: a directory that may hold children.
type Folder struct {
	Name     string
	Children []Folder
}

// Template is the set of top-level folders created for a project (SPEC §8.1).
// It is data, not hardcoded paths scattered through the code, so the default tree
// can be adjusted (or made operator-configurable) in one place.
type Template struct {
	Folders []Folder
}

// ScaffoldSpec describes one project tree to create.
type ScaffoldSpec struct {
	Base        string            // writable base directory (the only writable root)
	ProjectName string            // project subfolder name (sanitized before use)
	Mode        string            // creator mode, informational (Template already reflects it)
	Template    Template          // folder tree to create
	Docs        map[string]string // optional filename -> contents, written into DocsFolder
}

// Result reports what a scaffold created.
type Result struct {
	ProjectRoot string   // absolute project root created
	Dirs        []string // directories created, project-root-relative, in creation order
	Docs        []string // document filenames written, project-root-relative
}

// FSScaffolder is the filesystem Scaffolder — no Resolve or external dependency.
type FSScaffolder struct{}

// Scaffold creates the project folder tree under spec.Base and writes any Docs.
// The project name is sanitized and the result is confined to spec.Base so a
// crafted name cannot escape the writable base (SPEC §8.1 traversal guard).
func (FSScaffolder) Scaffold(spec ScaffoldSpec) (Result, error) {
	base := filepath.Clean(spec.Base)
	if base == "" || base == "." {
		return Result{}, errors.New("scaffold: base directory is required")
	}

	root, err := projectRoot(base, spec.ProjectName)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Result{}, fmt.Errorf("scaffold: create project root: %w", err)
	}

	res := Result{ProjectRoot: root}
	for _, f := range spec.Template.Folders {
		if err := createFolder(root, "", f, &res.Dirs); err != nil {
			return Result{}, err
		}
	}

	if len(spec.Docs) > 0 {
		docsDir := filepath.Join(root, DocsFolder)
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("scaffold: create docs dir: %w", err)
		}
		// Sorted keys keep output deterministic (and testable).
		names := make([]string, 0, len(spec.Docs))
		for name := range spec.Docs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			clean := sanitizeFileName(name)
			if clean == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(docsDir, clean), []byte(spec.Docs[name]), 0o644); err != nil {
				return Result{}, fmt.Errorf("scaffold: write doc %q: %w", clean, err)
			}
			res.Docs = append(res.Docs, filepath.Join(DocsFolder, clean))
		}
	}

	return res, nil
}

// createFolder makes one folder and recurses into its children, recording each
// created directory relative to the project root.
func createFolder(root, relParent string, f Folder, dirs *[]string) error {
	name := sanitizeFileName(f.Name)
	if name == "" {
		return nil
	}
	rel := filepath.Join(relParent, name)
	if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
		return fmt.Errorf("scaffold: create %q: %w", rel, err)
	}
	*dirs = append(*dirs, rel)
	for _, c := range f.Children {
		if err := createFolder(root, rel, c, dirs); err != nil {
			return err
		}
	}
	return nil
}

// projectRoot joins a sanitized project name onto base and verifies the result
// stays within base (defence in depth on top of name sanitization).
func projectRoot(base, name string) (string, error) {
	clean := SanitizeProjectName(name)
	if clean == "" {
		return "", errors.New("scaffold: project name is empty after sanitization")
	}
	root := filepath.Clean(filepath.Join(base, clean))
	rel, err := filepath.Rel(base, root)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("scaffold: project name %q escapes the base directory", name)
	}
	return root, nil
}

// SanitizeProjectName reduces an arbitrary title to a safe single path segment:
// path separators and control/reserved characters become "-", runs collapse, and
// leading/trailing separators and dots are trimmed (so "..", "/etc" cannot escape).
func SanitizeProjectName(s string) string {
	return sanitizeFileName(s)
}

// sanitizeFileName maps any character outside a conservative allowlist to "-",
// collapses repeats, and trims separators/dots from the ends.
func sanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	// Collapse runs of separators, then trim separators/dots from both ends.
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-_. ")
	return out
}

// DefaultTemplate returns the illustrative default project tree (SPEC §8.1) with
// footage subfolders chosen for the creator mode.
func DefaultTemplate(mode string) Template {
	return Template{Folders: []Folder{
		{Name: "01_Footage", Children: footageFolders(mode)},
		{Name: "02_Audio", Children: leaves("VO", "Music", "SFX")},
		{Name: "03_Graphics", Children: leaves("Thumbnails", "Lower-Thirds", "Overlays")},
		{Name: "04_Project"},
		{Name: "05_Exports"},
		{Name: "06_Assets"},
		{Name: DocsFolder},
	}}
}

// footageFolders picks the 01_Footage subfolders for a creator mode
// (store.CreatorModes: faceless, single_cam, multi_cam, obs).
func footageFolders(mode string) []Folder {
	switch mode {
	case "faceless":
		return leaves("Screen-Recordings", "Stock", "B-Roll")
	case "single_cam":
		return leaves("A-Cam", "B-Roll")
	case "multi_cam":
		return leaves("Cam-A", "Cam-B", "Cam-C", "B-Roll")
	case "obs":
		return leaves("Screen-Recordings", "Webcam", "B-Roll")
	default:
		return leaves("A-Cam", "B-Roll")
	}
}

// leaves builds childless folders from names.
func leaves(names ...string) []Folder {
	out := make([]Folder, 0, len(names))
	for _, n := range names {
		out = append(out, Folder{Name: n})
	}
	return out
}
