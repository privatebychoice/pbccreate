package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Thumbnail errors.
var (
	// ErrThumbUnavailable is returned when the ffmpeg binary cannot be found.
	ErrThumbUnavailable = errors.New("ffmpeg not available")
	// ErrThumbUnsupportedKind is returned for kinds without a still preview
	// (e.g. audio, other).
	ErrThumbUnsupportedKind = errors.New("kind does not support a preview frame")
)

// thumbWidth is the preview width in pixels; height auto-scales to preserve
// aspect ratio.
const thumbWidth = 480

// ThumbAvailable reports whether the configured ffmpeg binary is on PATH.
func ThumbAvailable(ffmpegPath string) bool {
	_, err := exec.LookPath(ffmpegBin(ffmpegPath))
	return err == nil
}

// GenerateThumbnail writes a JPEG preview of srcPath to destPath using ffmpeg.
// For video it grabs a single frame at seekSeconds; for image it rescales.
// Other kinds return ErrThumbUnsupportedKind, and a missing binary returns
// ErrThumbUnavailable, so callers can degrade gracefully.
func GenerateThumbnail(ctx context.Context, ffmpegPath, srcPath, destPath, kind string, seekSeconds int) error {
	bin := ffmpegBin(ffmpegPath)
	if _, err := exec.LookPath(bin); err != nil {
		return ErrThumbUnavailable
	}

	var args []string
	switch kind {
	case "video":
		args = []string{
			"-nostdin", "-y",
			"-ss", strconv.Itoa(seekSeconds),
			"-i", srcPath,
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:-1", thumbWidth),
			"-q:v", "3",
			destPath,
		}
	case "image":
		args = []string{
			"-nostdin", "-y",
			"-i", srcPath,
			"-vf", fmt.Sprintf("scale=%d:-1", thumbWidth),
			"-q:v", "3",
			destPath,
		}
	default:
		return ErrThumbUnsupportedKind
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		return fmt.Errorf("create preview dir: %w", err)
	}

	// Argument slice (never a shell string): no injection surface.
	cmd := exec.CommandContext(ctx, bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run ffmpeg: %w: %s", err, out)
	}
	if fi, err := os.Stat(destPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("ffmpeg produced no preview for %q", srcPath)
	}
	return nil
}

func ffmpegBin(ffmpegPath string) string {
	if ffmpegPath == "" {
		return "ffmpeg"
	}
	return ffmpegPath
}
