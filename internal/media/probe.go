package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// ErrProbeUnavailable is returned when the ffprobe binary cannot be found.
var ErrProbeUnavailable = errors.New("ffprobe not available")

// Metadata is the subset of technical media properties pbccreate records
// (SPEC §5.7). Zero-valued fields mean "unknown".
type Metadata struct {
	DurationSeconds int
	Width           int
	Height          int
	Codec           string
	FPS             float64
	Container       string
}

// ProbeAvailable reports whether the configured ffprobe binary is on PATH.
func ProbeAvailable(ffprobePath string) bool {
	_, err := exec.LookPath(probeBin(ffprobePath))
	return err == nil
}

// Probe runs ffprobe on filePath and returns its media metadata. It returns
// ErrProbeUnavailable if the binary is missing, so callers can degrade
// gracefully.
func Probe(ctx context.Context, ffprobePath, filePath string) (Metadata, error) {
	bin := probeBin(ffprobePath)
	if _, err := exec.LookPath(bin); err != nil {
		return Metadata{}, ErrProbeUnavailable
	}
	// Argument slice (never a shell string): no injection surface.
	cmd := exec.CommandContext(ctx, bin,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("run ffprobe: %w", err)
	}
	return parseProbeOutput(out)
}

func probeBin(ffprobePath string) string {
	if ffprobePath == "" {
		return "ffprobe"
	}
	return ffprobePath
}

// ffprobe JSON shapes (only the fields we use).
type ffprobeOutput struct {
	Format struct {
		Duration   string `json:"duration"`
		FormatName string `json:"format_name"`
	} `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	RFrameRate   string `json:"r_frame_rate"`
	AvgFrameRate string `json:"avg_frame_rate"`
}

// parseProbeOutput extracts Metadata from ffprobe's JSON. Kept separate from the
// exec call so it can be unit-tested without the binary.
func parseProbeOutput(data []byte) (Metadata, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return Metadata{}, fmt.Errorf("parse ffprobe json: %w", err)
	}

	var m Metadata
	if d, err := strconv.ParseFloat(strings.TrimSpace(out.Format.Duration), 64); err == nil && d > 0 {
		m.DurationSeconds = int(math.Round(d))
	}
	m.Container = firstToken(out.Format.FormatName)

	var video, audio *ffprobeStream
	for i := range out.Streams {
		s := &out.Streams[i]
		if s.CodecType == "video" && video == nil {
			video = s
		}
		if s.CodecType == "audio" && audio == nil {
			audio = s
		}
	}
	switch {
	case video != nil:
		m.Width = video.Width
		m.Height = video.Height
		m.Codec = video.CodecName
		m.FPS = parseFPS(video.RFrameRate)
		if m.FPS == 0 {
			m.FPS = parseFPS(video.AvgFrameRate)
		}
	case audio != nil:
		m.Codec = audio.CodecName
	}
	return m, nil
}

// parseFPS parses a rate like "30000/1001" or "25" into frames per second.
func parseFPS(s string) float64 {
	s = strings.TrimSpace(s)
	if num, den, ok := strings.Cut(s, "/"); ok {
		n, err1 := strconv.ParseFloat(num, 64)
		d, err2 := strconv.ParseFloat(den, 64)
		if err1 == nil && err2 == nil && d != 0 {
			return n / d
		}
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// firstToken returns the text before the first comma (ffprobe format_name is
// often a comma list, e.g. "mov,mp4,m4a,3gp,3g2,mj2").
func firstToken(s string) string {
	if i := strings.IndexByte(s, ','); i >= 0 {
		return s[:i]
	}
	return s
}
