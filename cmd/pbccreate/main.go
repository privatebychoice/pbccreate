// Command pbccreate is a local-only, privacy-first content-planning tool for
// creators. See docs/SPEC.md for the full specification.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.privatebychoice.com/pbccreate/internal/buildinfo"
	"go.privatebychoice.com/pbccreate/internal/config"
)

func main() {
	log := newLogger()

	// Dispatch: first non-flag argument is the subcommand; default is "serve".
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	var err error
	switch cmd {
	case "version":
		fmt.Println("pbccreate " + buildinfo.String())
	case "serve":
		err = runServe(log)
	case "scaffold":
		err = runScaffold(log)
	case "help", "-h", "--help":
		usage()
	default:
		log.Error("unknown command", "command", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Error("command failed", "command", cmd, "err", err)
		os.Exit(1)
	}
}

// runServe will host the loopback web UI. The HTTP server itself arrives in a
// later slice; for now it resolves and reports configuration so the wiring can
// be reviewed.
func runServe(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Log(log)
	log.Warn("serve: HTTP server not yet implemented (upcoming slice)")
	return nil
}

// runScaffold will create DaVinci Resolve project folders (docs/SPEC.md §8.1).
// Arrives in a later slice.
func runScaffold(log *slog.Logger) error {
	log.Warn("scaffold: not yet implemented (upcoming slice)")
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `pbccreate — local content-planning tool for creators

Usage:
  pbccreate [command]

Commands:
  serve      Start the local web UI (default)
  scaffold   Create DaVinci Resolve project folders
  version    Print version and build number
  help       Show this help
`)
}

// newLogger builds a structured stderr logger. Level is INFO by default and can
// be raised/lowered via PBCCREATE_LOG (debug|info|warn|error).
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("PBCCREATE_LOG"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
