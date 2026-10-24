// Package logx opens the sidecar's rotating text log.
package logx

import (
	"io"
	"log/slog"
	"os"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

const maxSize = 1 << 20

// Open returns a logger writing to paths.LogPath(). It rotates pcs.log to
// pcs.log.1 when over 1 MiB (keeping one old file) and falls back to a
// discard logger if the log cannot be opened. It never fails.
func Open() *slog.Logger {
	p := paths.LogPath()
	if err := os.MkdirAll(paths.LogDir(), 0o755); err != nil {
		return discard()
	}
	if fi, err := os.Stat(p); err == nil && fi.Size() > maxSize {
		_ = os.Rename(p, p+".1")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return discard()
	}
	return slog.New(slog.NewTextHandler(f, nil))
}

func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
