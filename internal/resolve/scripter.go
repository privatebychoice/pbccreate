package resolve

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// helperPy is the DaVinci Resolve scripting helper, embedded so the binary stays
// self-contained (SPEC §8.2). It is materialized to a temp file per invocation
// and driven over a JSON stdin/stdout boundary.
//
//go:embed helper.py
var helperPy []byte

// Scripter drives Resolve Studio through the external Python helper. Every method
// performs one helper invocation; the helper reports Resolve-side failures in the
// Response (OK=false) rather than as a Go error, so local state is never touched.
type Scripter interface {
	// Ping checks that Resolve is running and reachable.
	Ping(ctx context.Context) (Response, error)
	// CreateProject opens the named project or creates it if absent.
	CreateProject(ctx context.Context, spec ProjectSpec) (Response, error)
	// ImportMedia imports clips into named media-pool bins.
	ImportMedia(ctx context.Context, bins []Bin) (Response, error)
	// BuildTimeline creates a timeline and appends clips in the given order.
	BuildTimeline(ctx context.Context, tl Timeline) (Response, error)
}

// ProjectSpec names a Resolve project (and its optional on-disk location, the
// scaffolded 04_Project context).
type ProjectSpec struct {
	Name     string `json:"name"`
	Location string `json:"location,omitempty"`
}

// Bin is a media-pool folder and the clip file paths to import into it.
type Bin struct {
	Name  string   `json:"name"`
	Clips []string `json:"clips,omitempty"`
}

// Timeline describes a timeline to build from the shot list (SPEC §5.3/§8.2).
// Clips are absolute file paths in shot order. For Multi Cam, CameraBins groups
// clip paths per camera to support downstream multicam sync.
type Timeline struct {
	Name       string              `json:"name"`
	Clips      []string            `json:"clips,omitempty"`
	Multicam   bool                `json:"multicam,omitempty"`
	CameraBins map[string][]string `json:"camera_bins,omitempty"`
}

// command is the JSON envelope written to the helper's stdin.
type command struct {
	Action   string       `json:"action"`
	Project  *ProjectSpec `json:"project,omitempty"`
	Bins     []Bin        `json:"bins,omitempty"`
	Timeline *Timeline    `json:"timeline,omitempty"`
}

// Response is the JSON result read from the helper's stdout. OK=false with a
// populated Error means the call ran but Resolve reported a problem (e.g. not
// running); that is distinct from a Go transport error.
type Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// pythonScripter runs the embedded helper via python3 (SPEC §8.2). The Resolve
// environment (RESOLVE_SCRIPT_API/LIB, PYTHONPATH) is inherited from env.
type pythonScripter struct {
	python string   // python3 executable (config.Python)
	env    []string // process environment passed to the helper
	helper []byte   // helper source materialized per invocation
}

func newPythonScripter(python string) pythonScripter {
	if python == "" {
		python = "python3"
	}
	return pythonScripter{python: python, env: os.Environ(), helper: helperPy}
}

func (s pythonScripter) Ping(ctx context.Context) (Response, error) {
	return s.run(ctx, command{Action: "ping"})
}

func (s pythonScripter) CreateProject(ctx context.Context, spec ProjectSpec) (Response, error) {
	return s.run(ctx, command{Action: "create_project", Project: &spec})
}

func (s pythonScripter) ImportMedia(ctx context.Context, bins []Bin) (Response, error) {
	return s.run(ctx, command{Action: "import_media", Bins: bins})
}

func (s pythonScripter) BuildTimeline(ctx context.Context, tl Timeline) (Response, error) {
	return s.run(ctx, command{Action: "build_timeline", Timeline: &tl})
}

// run materializes the helper to a temp file, invokes python3 with an argument
// slice (never a shell string), pipes the command JSON on stdin, and decodes the
// JSON response from stdout.
func (s pythonScripter) run(ctx context.Context, cmd command) (Response, error) {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return Response{}, fmt.Errorf("resolve: marshal command: %w", err)
	}

	f, err := os.CreateTemp("", "pbccreate-resolve-*.py")
	if err != nil {
		return Response{}, fmt.Errorf("resolve: materialize helper: %w", err)
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := f.Write(s.helper); err != nil {
		_ = f.Close()
		return Response{}, fmt.Errorf("resolve: write helper: %w", err)
	}
	if err := f.Close(); err != nil {
		return Response{}, fmt.Errorf("resolve: close helper: %w", err)
	}

	python := s.python
	if python == "" {
		python = "python3"
	}
	c := exec.CommandContext(ctx, python, name)
	if s.env != nil {
		c.Env = s.env
	}
	var stdout, stderr bytes.Buffer
	c.Stdin = bytes.NewReader(payload)
	c.Stdout = &stdout
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		return Response{}, fmt.Errorf("resolve: helper exec failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var resp Response
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &resp); err != nil {
		return Response{}, fmt.Errorf("resolve: decode helper response: %w (output: %s)", err, truncateOutput(stdout.String()))
	}
	return resp, nil
}

// truncateOutput bounds helper output included in error messages.
func truncateOutput(s string) string {
	s = strings.TrimSpace(s)
	const max = 200
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
