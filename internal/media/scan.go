package media

import (
	"io/fs"
	"path/filepath"
	"sort"
	"time"
)

// ScannedFile describes a media file discovered by ScanDir.
type ScannedFile struct {
	Path    string
	Kind    string
	Size    int64
	ModTime time.Time
}

// ScanDir walks root recursively and returns the regular files whose extension
// maps to a recognized media kind (video/audio/image); "other" files are
// skipped. Unreadable entries are skipped rather than aborting the walk. Results
// are sorted by path.
func ScanDir(root string) ([]ScannedFile, error) {
	var out []ScannedFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we cannot read
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		k := Kind(path)
		if k == "other" {
			return nil
		}
		if info := Stat(path); info.Exists {
			out = append(out, ScannedFile{Path: path, Kind: k, Size: info.Size, ModTime: info.ModTime})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
