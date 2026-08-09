package media

import (
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateThumbnailUnavailable(t *testing.T) {
	err := GenerateThumbnail(context.Background(), "pbccreate-no-such-ffmpeg-xyz",
		"/tmp/in.mp4", "/tmp/out.jpg", "video", 0)
	if err != ErrThumbUnavailable {
		t.Errorf("err = %v, want ErrThumbUnavailable", err)
	}
}

func TestGenerateThumbnailUnsupportedKind(t *testing.T) {
	if !ThumbAvailable("") {
		t.Skip("ffmpeg not installed; skipping")
	}
	err := GenerateThumbnail(context.Background(), "", "/tmp/in.wav", "/tmp/out.jpg", "audio", 0)
	if err != ErrThumbUnsupportedKind {
		t.Errorf("err = %v, want ErrThumbUnsupportedKind", err)
	}
}

// TestGenerateThumbnailIntegration generates a real clip and thumbnails it.
// Skipped unless ffmpeg is installed.
func TestGenerateThumbnailIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; skipping integration test")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	gen := exec.Command("ffmpeg", "-nostdin", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=640x360:rate=30",
		"-pix_fmt", "yuv420p", src)
	if b, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate: %v\n%s", err, b)
	}

	dest := filepath.Join(dir, "preview.jpg")
	if err := GenerateThumbnail(context.Background(), "", src, dest, "video", 0); err != nil {
		t.Fatalf("GenerateThumbnail: %v", err)
	}

	f, err := os.Open(dest)
	if err != nil {
		t.Fatalf("open preview: %v", err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatalf("preview is not a valid JPEG: %v", err)
	}
	if cfg.Width != thumbWidth {
		t.Errorf("preview width = %d, want %d", cfg.Width, thumbWidth)
	}
}
