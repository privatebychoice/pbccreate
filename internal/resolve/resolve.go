// Package resolve implements pbccreate's DaVinci Resolve integration (SPEC §8).
//
// Two halves sit behind one boundary so callers depend on the interface, not the
// Resolve edition:
//
//   - Scaffolder — pure filesystem work, always available: it creates a project
//     folder tree (per creator mode) and optionally writes plan documents into
//     the Docs subfolder. No Resolve, no external dependency (SPEC §8.1).
//   - Scripter — drives Resolve Studio over an external Python 3 helper. It is
//     optional and runtime-detected; when unavailable the app uses the Scaffolder
//     only and surfaces a clear message (SPEC §8.2). The Scripter itself lands in
//     a later slice; this slice provides the interface boundary and the runtime
//     prerequisite detection.
package resolve

import (
	"os"
	"os/exec"
	"strings"
)

// DocsFolder is the subfolder that receives exported plan documents (SPEC §8.1).
const DocsFolder = "07_Docs"

// Scaffolder creates a Resolve project folder tree on disk. Always available.
type Scaffolder interface {
	Scaffold(spec ScaffoldSpec) (Result, error)
}

// Integration is the ResolveIntegration boundary (SPEC §8): it bundles the
// always-available Scaffolder with the optional, runtime-detected Scripter.
type Integration struct {
	scaffolder Scaffolder
	scripter   Scripter
	scripting  ScriptingStatus
}

// New builds an Integration. The Scripter's prerequisites are probed once from
// the process environment and the configured python executable; the Scripter
// itself is only offered to callers when those prerequisites are present.
func New(python string) *Integration {
	return &Integration{
		scaffolder: FSScaffolder{},
		scripter:   newPythonScripter(python),
		scripting:  DetectScripting(os.Getenv, python, exec.LookPath),
	}
}

// Scaffolder returns the always-available filesystem scaffolder.
func (i *Integration) Scaffolder() Scaffolder { return i.scaffolder }

// Scripter returns the Resolve scripter and whether its prerequisites are
// present. When false, callers should fall back to the Scaffolder and surface
// Scripting().Reason rather than attempting to script (SPEC §8.2).
func (i *Integration) Scripter() (Scripter, bool) { return i.scripter, i.scripting.Available }

// Scripting reports whether the Resolve scripting prerequisites are present.
func (i *Integration) Scripting() ScriptingStatus { return i.scripting }

// ScriptingStatus is the result of probing the Resolve scripting prerequisites.
type ScriptingStatus struct {
	Available bool     // all prerequisites present
	Reason    string   // human-readable explanation (always set)
	Missing   []string // which prerequisites were absent (nil when Available)
}

// scriptEnvVars are the environment variables Blackmagic's DaVinciResolveScript
// module needs to locate the Studio scripting API (SPEC §8.2).
var scriptEnvVars = []string{"RESOLVE_SCRIPT_API", "RESOLVE_SCRIPT_LIB", "PYTHONPATH"}

// DetectScripting probes the prerequisites for external Resolve scripting: the
// three RESOLVE_* / PYTHONPATH environment variables and a resolvable Python 3.
//
// It is deliberately dependency-injected (getenv, lookPath) so it is testable
// without touching the real environment. This checks prerequisites only; the
// final liveness check — Resolve Studio actually running and scriptapp("Resolve")
// returning non-nil — happens when the Python helper runs (a later slice).
func DetectScripting(getenv func(string) string, python string, lookPath func(string) (string, error)) ScriptingStatus {
	var missing []string
	for _, k := range scriptEnvVars {
		if strings.TrimSpace(getenv(k)) == "" {
			missing = append(missing, k)
		}
	}
	if python == "" {
		python = "python3"
	}
	if _, err := lookPath(python); err != nil {
		missing = append(missing, "python3 ("+python+" not found in PATH)")
	}

	if len(missing) == 0 {
		return ScriptingStatus{
			Available: true,
			Reason:    "Resolve scripting prerequisites present (Studio must be running when a script executes).",
		}
	}
	return ScriptingStatus{
		Available: false,
		Missing:   missing,
		Reason:    "scripting requires DaVinci Resolve Studio + Python 3; missing: " + strings.Join(missing, ", "),
	}
}
