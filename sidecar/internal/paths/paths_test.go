package paths

import "testing"

func TestPathsUnderHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cases := map[string][2]string{
		"launcher": {LauncherPath(), dir + "/.pcs/bin/pcs-hook"},
		"socket":   {SocketPath(), dir + "/.pcs/run/app.sock"},
		"appbin":   {AppBinary(), dir + "/Applications/Personalization Setup.app/Contents/MacOS/pcs"},
		"log":      {LogPath(), dir + "/Library/Logs/Personalization Setup/pcs.log"},
		"settings": {SettingsPath(), dir + "/.claude/settings.json"},
		"state":    {StatePath(), dir + "/.pcs/state.json"},
		"backups":  {BackupDir(), dir + "/.pcs/backups"},
	}
	for name, c := range cases {
		if c[0] != c[1] {
			t.Errorf("%s: got %q want %q", name, c[0], c[1])
		}
	}
}
