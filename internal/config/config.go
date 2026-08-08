// Package config loads pbccreate's runtime configuration from environment
// variables with OS-appropriate defaults. Nothing environment-specific is
// compiled in (see docs/SPEC.md §2 and §12).
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const appName = "pbccreate"

// defaultAddr binds the local web UI to loopback only (see docs/SPEC.md §2).
const defaultAddr = "127.0.0.1:8787"

// Config holds resolved runtime settings.
type Config struct {
	DataDir   string // SQLite DB + local state
	ConfigDir string // config files

	MediaRoots []string // catalogued media locations
	AssetRoot  string   // cross-project asset-library root (optional, §5.16)

	Addr string // loopback listen address

	// External tool invocations (path or bare name resolved from PATH at use).
	FFprobe string // media metadata (§5.7)
	FFmpeg  string // preview-frame extraction (§5.7)
	Python  string // python3 for the Resolve scripting helper (§8.2)

	// NetworkEnabled is the master network-egress switch. Default-deny: false
	// means the app opens no outbound connections (see docs/SPEC.md §9.1).
	NetworkEnabled bool
}

// Load resolves configuration from the environment, applying defaults.
func Load() (*Config, error) {
	dataDir, err := resolveDir("PBCCREATE_DATA_DIR", "XDG_DATA_HOME", filepath.Join(".local", "share"))
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	configDir, err := resolveDir("PBCCREATE_CONFIG_DIR", "XDG_CONFIG_HOME", ".config")
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}

	return &Config{
		DataDir:        dataDir,
		ConfigDir:      configDir,
		MediaRoots:     splitPaths(os.Getenv("PBCCREATE_MEDIA_ROOTS")),
		AssetRoot:      os.Getenv("PBCCREATE_ASSET_ROOT"),
		Addr:           envOr("PBCCREATE_ADDR", defaultAddr),
		FFprobe:        envOr("PBCCREATE_FFPROBE", "ffprobe"),
		FFmpeg:         envOr("PBCCREATE_FFMPEG", "ffmpeg"),
		Python:         envOr("PBCCREATE_PYTHON", "python3"),
		NetworkEnabled: envBool("PBCCREATE_NETWORK", false),
	}, nil
}

// Log emits the resolved configuration. No secrets are stored in Config, so this
// is safe to log (see docs/SPEC.md §9 logging rules).
func (c *Config) Log(log *slog.Logger) {
	log.Info("configuration resolved",
		"data_dir", c.DataDir,
		"config_dir", c.ConfigDir,
		"media_roots", c.MediaRoots,
		"asset_root", c.AssetRoot,
		"addr", c.Addr,
		"network_enabled", c.NetworkEnabled,
	)
	log.Debug("external tool configuration",
		"ffprobe", c.FFprobe, "ffmpeg", c.FFmpeg, "python", c.Python)
}

// resolveDir returns, in order of precedence:
//   - $<overrideEnv> verbatim, else
//   - $<xdgEnv>/pbccreate, else
//   - $HOME/<fallback>/pbccreate.
func resolveDir(overrideEnv, xdgEnv, fallback string) (string, error) {
	if v := os.Getenv(overrideEnv); v != "" {
		return v, nil
	}
	if v := os.Getenv(xdgEnv); v != "" {
		return filepath.Join(v, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fallback, appName), nil
}

// splitPaths splits an OS path-list (":" on Unix) into trimmed, non-empty entries.
func splitPaths(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, string(os.PathListSeparator)) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
