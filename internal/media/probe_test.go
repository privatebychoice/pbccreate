package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseProbeOutput(t *testing.T) {
	const sample = `{
	  "streams": [
	    {"codec_type": "video", "codec_name": "h264", "width": 1920, "height": 1080, "r_frame_rate": "30000/1001", "avg_frame_rate": "30000/1001"},
	    {"codec_type": "audio", "codec_name": "aac"}
	  ],
	  "format": {"duration": "12.480000", "format_name": "mov,mp4,m4a,3gp,3g2,mj2"}
	}`

	m, err := parseProbeOutput([]byte(sample))
	if err != nil {
		t.Fatalf("parseProbeOutput: %v", err)
	}
	if m.DurationSeconds != 12 {
		t.Errorf("DurationSeconds = %d, want 12", m.DurationSeconds)
	}
	if m.Width != 1920 || m.Height != 1080 {
		t.Errorf("resolution = %dx%d, want 1920x1080", m.Width, m.Height)
	}
	if m.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", m.Codec)
	}
	if m.Container != "mov" {
		t.Errorf("Container = %q, want mov", m.Container)
	}
	if got := m.FPS; got < 29.9 || got > 30.0 {
		t.Errorf("FPS = %v, want ~29.97", got)
	}
}

func TestParseProbeOutputAudioOnly(t *testing.T) {
	const sample = `{"streams":[{"codec_type":"audio","codec_name":"flac"}],"format":{"duration":"3.5","format_name":"flac"}}`
	m, err := parseProbeOutput([]byte(sample))
	if err != nil {
		t.Fatalf("parseProbeOutput: %v", err)
	}
	if m.Codec != "flac" || m.Width != 0 || m.DurationSeconds != 4 {
		t.Errorf("unexpected: %+v", m)
	}
}

func TestParseFPS(t *testing.T) {
	cases := map[string]float64{"25": 25, "30000/1001": 29.97, "0/0": 0, "": 0}
	for in, want := range cases {
		got := parseFPS(in)
		if want == 0 && got != 0 {
			t.Errorf("parseFPS(%q) = %v, want 0", in, got)
		}
		if want != 0 && (got < want-0.01 || got > want+0.01) {
			t.Errorf("parseFPS(%q) = %v, want ~%v", in, got, want)
		}
	}
}

func TestProbeUnavailable(t *testing.T) {
	_, err := Probe(context.Background(), "pbccreate-no-such-binary-xyz", "/tmp/whatever.mp4")
	if err != ErrProbeUnavailable {
		t.Errorf("err = %v, want ErrProbeUnavailable", err)
	}
}

// TestProbeIntegration generates a tiny video with ffmpeg and probes it. It is
// skipped unless both ffmpeg and ffprobe are installed.
func TestProbeIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; skipping integration test")
	}
	if !ProbeAvailable("") {
		t.Skip("ffprobe not installed; skipping integration test")
	}

	out := filepath.Join(t.TempDir(), "test.mp4")
	gen := exec.Command("ffmpeg", "-nostdin", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=30",
		"-pix_fmt", "yuv420p", out)
	if b, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate: %v\n%s", err, b)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("generated file missing: %v", err)
	}

	m, err := Probe(context.Background(), "", out)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if m.Width != 320 || m.Height != 240 {
		t.Errorf("resolution = %dx%d, want 320x240", m.Width, m.Height)
	}
	if m.DurationSeconds != 1 {
		t.Errorf("DurationSeconds = %d, want 1", m.DurationSeconds)
	}
	if m.Codec == "" {
		t.Error("codec empty")
	}
}
