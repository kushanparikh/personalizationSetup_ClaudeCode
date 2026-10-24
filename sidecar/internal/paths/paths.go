// Package paths is the single place that names the app and its files.
package paths

import (
	"os"
	"path/filepath"
)

const (
	AppName     = "Personalization Setup"
	BundleID    = "com.kushanparikh.personalizationsetup"
	SidecarName = "pcs"

	// LauncherCommand is the hook command prefix as written into settings.json.
	LauncherCommand = "~/.pcs/bin/pcs-hook"
)

// Home returns the user's home directory.
func Home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func AppPath() string      { return filepath.Join(Home(), "Applications", AppName+".app") }
func AppBinary() string    { return filepath.Join(AppPath(), "Contents", "MacOS", SidecarName) }
func PCSDir() string       { return filepath.Join(Home(), ".pcs") }
func LauncherPath() string { return filepath.Join(PCSDir(), "bin", "pcs-hook") }
func SocketPath() string   { return filepath.Join(PCSDir(), "run", "app.sock") }
func StatePath() string    { return filepath.Join(PCSDir(), "state.json") }
func BackupDir() string    { return filepath.Join(PCSDir(), "backups") }
func LogDir() string       { return filepath.Join(Home(), "Library", "Logs", AppName) }
func LogPath() string      { return filepath.Join(LogDir(), "pcs.log") }
func SettingsPath() string { return filepath.Join(Home(), ".claude", "settings.json") }
