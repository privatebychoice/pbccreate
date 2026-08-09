package resolve

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// fakePython writes an executable shell stand-in for python3. It is invoked as
// "<script> <helperfile>"; the body reads the command JSON on stdin and writes a
// response on stdout, exactly like the real helper's boundary.
func fakePython(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-interpreter tests use a POSIX shell script")
	}
	p := filepath.Join(t.TempDir(), "fakepython.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScripterSerializesAndParses(t *testing.T) {
	dir := t.TempDir()
	capFile := filepath.Join(dir, "cap.json")
	respFile := filepath.Join(dir, "resp.json")
	if err := os.WriteFile(respFile, []byte(`{"ok":true,"message":"imported 1 clip(s)"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := fakePython(t, `cat > "$PBC_CAPTURE"`+"\n"+`cat "$PBC_RESPONSE"`+"\n")
	s := pythonScripter{
		python: fake,
		env:    append(os.Environ(), "PBC_CAPTURE="+capFile, "PBC_RESPONSE="+respFile),
		helper: helperPy,
	}

	resp, err := s.ImportMedia(context.Background(), []Bin{{Name: "Cam-A", Clips: []string{"/a.mov", "/b.mov"}}})
	if err != nil {
		t.Fatalf("ImportMedia: %v", err)
	}
	if !resp.OK || resp.Message != "imported 1 clip(s)" {
		t.Fatalf("resp = %+v", resp)
	}

	// The command JSON reaching the helper must be well-formed and complete.
	raw, err := os.ReadFile(capFile)
	if err != nil {
		t.Fatal(err)
	}
	var cmd command
	if err := json.Unmarshal(raw, &cmd); err != nil {
		t.Fatalf("captured command is not valid JSON: %s", raw)
	}
	if cmd.Action != "import_media" || len(cmd.Bins) != 1 {
		t.Fatalf("captured command = %+v", cmd)
	}
	if cmd.Bins[0].Name != "Cam-A" || len(cmd.Bins[0].Clips) != 2 || cmd.Bins[0].Clips[1] != "/b.mov" {
		t.Errorf("bin serialization wrong: %+v", cmd.Bins[0])
	}
}

func TestScripterBuildTimelineSerialization(t *testing.T) {
	dir := t.TempDir()
	capFile := filepath.Join(dir, "cap.json")
	respFile := filepath.Join(dir, "resp.json")
	if err := os.WriteFile(respFile, []byte(`{"ok":true,"message":"ok"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := fakePython(t, `cat > "$PBC_CAPTURE"`+"\n"+`cat "$PBC_RESPONSE"`+"\n")
	s := pythonScripter{
		python: fake,
		env:    append(os.Environ(), "PBC_CAPTURE="+capFile, "PBC_RESPONSE="+respFile),
		helper: helperPy,
	}

	tl := Timeline{
		Name:       "Ep 1",
		Clips:      []string{"/1.mov", "/2.mov"},
		Multicam:   true,
		CameraBins: map[string][]string{"Cam-A": {"/1.mov"}, "Cam-B": {"/2.mov"}},
	}
	if _, err := s.BuildTimeline(context.Background(), tl); err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}

	raw, _ := os.ReadFile(capFile)
	var cmd command
	if err := json.Unmarshal(raw, &cmd); err != nil {
		t.Fatalf("captured not JSON: %s", raw)
	}
	if cmd.Action != "build_timeline" || cmd.Timeline == nil {
		t.Fatalf("captured = %+v", cmd)
	}
	if !cmd.Timeline.Multicam || len(cmd.Timeline.Clips) != 2 || len(cmd.Timeline.CameraBins) != 2 {
		t.Errorf("timeline serialization wrong: %+v", cmd.Timeline)
	}
}

func TestScripterSurfacesResolveFailure(t *testing.T) {
	dir := t.TempDir()
	respFile := filepath.Join(dir, "resp.json")
	if err := os.WriteFile(respFile, []byte(`{"ok":false,"error":"Resolve is not running","code":"no_resolve"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := fakePython(t, `cat > /dev/null`+"\n"+`cat "$PBC_RESPONSE"`+"\n")
	s := pythonScripter{python: fake, env: append(os.Environ(), "PBC_RESPONSE="+respFile), helper: helperPy}

	resp, err := s.Ping(context.Background())
	if err != nil {
		t.Fatalf("transport error not expected: %v", err)
	}
	if resp.OK || resp.Code != "no_resolve" || resp.Error == "" {
		t.Errorf("expected a Resolve-side failure, got %+v", resp)
	}
}

func TestScripterTransportAndDecodeErrors(t *testing.T) {
	// Non-zero exit → transport error.
	bad := fakePython(t, `echo boom 1>&2`+"\n"+`exit 3`+"\n")
	s := pythonScripter{python: bad, env: os.Environ(), helper: helperPy}
	if _, err := s.Ping(context.Background()); err == nil {
		t.Error("expected a transport error on non-zero exit")
	}

	// Zero exit but non-JSON stdout → decode error.
	garble := fakePython(t, `cat > /dev/null`+"\n"+`printf 'not json'`+"\n")
	s2 := pythonScripter{python: garble, env: os.Environ(), helper: helperPy}
	if _, err := s2.Ping(context.Background()); err == nil {
		t.Error("expected a decode error on non-JSON output")
	}
}

func TestIntegrationScripterGating(t *testing.T) {
	// Prerequisites absent (bogus python) → Scripter not offered.
	if _, ok := New("pbccreate-no-such-python-xyz").Scripter(); ok {
		t.Error("expected scripter unavailable with a missing python")
	}

	// Prerequisites present (env + a resolvable executable) → offered.
	t.Setenv("RESOLVE_SCRIPT_API", "/x")
	t.Setenv("RESOLVE_SCRIPT_LIB", "/x")
	t.Setenv("PYTHONPATH", "/x")
	if _, ok := New("sh").Scripter(); !ok {
		t.Error("expected scripter available with env + resolvable interpreter")
	}
}

// TestHelperCompiles validates the embedded Python helper's syntax against the
// real interpreter when one is installed. The full Resolve round-trip cannot be
// exercised here (no Resolve Studio); this is the honest coverage available.
func TestHelperCompiles(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed; skipping helper syntax check")
	}
	p := filepath.Join(t.TempDir(), "helper.py")
	if err := os.WriteFile(p, helperPy, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(py, "-m", "py_compile", p).CombinedOutput(); err != nil {
		t.Fatalf("embedded helper.py does not compile: %v\n%s", err, out)
	}
}
