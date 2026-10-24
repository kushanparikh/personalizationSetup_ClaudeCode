// Package launcher installs the small shell script that settings.json hook
// commands point at. It lives outside the app bundle, so a deleted app makes
// every hook quietly do nothing instead of failing.
package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

const script = `#!/bin/sh
# Installed by @APP@. Runs the app's hook handler; if the app is gone, does nothing.
BIN="@BIN@"
if [ ! -x "$BIN" ]; then
  cat >/dev/null
  exit 0
fi
exec "$BIN" "$@"
`

// Script returns the launcher script for bin.
func Script(bin string) string {
	return strings.NewReplacer("@APP@", paths.AppName, "@BIN@", bin).Replace(script)
}

// Install writes the launcher for bin at path with mode 0755. It writes only
// if the content or mode differs, and reports whether it changed anything.
// bin must not contain characters that are special inside sh double quotes.
func Install(path, bin string) (changed bool, err error) {
	if err := checkBin(bin); err != nil {
		return false, err
	}
	want := []byte(Script(bin))
	cur, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err == nil && bytes.Equal(cur, want) {
		fi, err := os.Stat(path)
		if err != nil {
			return false, err
		}
		if fi.Mode().Perm() == 0o755 {
			return false, nil
		}
		return true, os.Chmod(path, 0o755)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(dir, ".pcs-hook-*")
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(0o755); err != nil {
		return false, err
	}
	if _, err = tmp.Write(want); err != nil {
		return false, err
	}
	if err = tmp.Sync(); err != nil {
		return false, err
	}
	if err = tmp.Close(); err != nil {
		return false, err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}

// EnsureInstalled writes the launcher only if nothing exists at path, so a
// launcher pointed elsewhere (e.g. a development build) is kept.
func EnsureInstalled(path, bin string) (changed bool, err error) {
	if err := checkBin(bin); err != nil {
		return false, err
	}
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return Install(path, bin)
}

func checkBin(bin string) error {
	if bin == "" || strings.ContainsAny(bin, "\"$`\\") {
		return fmt.Errorf("unsafe app path for the launcher: %q", bin)
	}
	return nil
}
