// Package media holds local-filesystem concerns for the media catalogue: kind
// inference, presence/stat checks, and media-root path validation. It performs no
// network access and does not depend on the database layer (see docs/SPEC.md
// §5.7, §9).
package media

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Info is the result of stat-ing a file on disk.
type Info struct {
	Exists  bool
	Size    int64
	ModTime time.Time
}

// Stat reports whether path is an existing regular file and, if so, its size and
// modification time. Directories and errors yield Exists=false.
func Stat(path string) Info {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return Info{}
	}
	return Info{Exists: true, Size: fi.Size(), ModTime: fi.ModTime().UTC()}
}

var kindByExt = map[string]string{
	".mp4": "video", ".mov": "video", ".mkv": "video", ".avi": "video",
	".webm": "video", ".m4v": "video", ".mpg": "video", ".mpeg": "video",
	".wav": "audio", ".mp3": "audio", ".aac": "audio", ".flac": "audio",
	".m4a": "audio", ".aiff": "audio", ".aif": "audio", ".ogg": "audio",
	".jpg": "image", ".jpeg": "image", ".png": "image", ".gif": "image",
	".webp": "image", ".tif": "image", ".tiff": "image", ".bmp": "image", ".heic": "image",
}

// Kind infers a media kind (video/audio/image/other) from a path's extension.
func Kind(path string) string {
	if k, ok := kindByExt[strings.ToLower(filepath.Ext(path))]; ok {
		return k
	}
	return "other"
}

// WithinRoots reports whether an absolute path lies inside one of the configured
// media roots. With no roots configured, any absolute path is accepted. Relative
// paths are always rejected.
func WithinRoots(path string, roots []string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	if len(roots) == 0 {
		return true
	}
	sep := string(filepath.Separator)
	for _, root := range roots {
		rel, err := filepath.Rel(filepath.Clean(root), clean)
		if err != nil {
			continue
		}
		// rel that is ".." or starts with "../" escapes the root.
		if rel == ".." || strings.HasPrefix(rel, ".."+sep) {
			continue
		}
		return true
	}
	return false
}
