package claude

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/agent"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/settings"
)

var _ agent.Adapter = Adapter{}

// Install replaces the owned entries of one hook type in settings.json with
// entries. The type is read from the entries' commands.
func (Adapter) Install(entries []hooktypes.Entry) error {
	t, err := typeOf(entries)
	if err != nil {
		return err
	}
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return err
	}
	f.SetOwned(t, entries)
	if !ownedMatch(f, t, entries) { // SetOwned refused a malformed shape
		d, _ := hooktypes.Get(t)
		return fmt.Errorf(`~/.claude/settings.json has a "hooks" section this app can't safely edit, so %s wasn't added. Nothing was changed.`, d.Name)
	}
	_, err = save(f, settings.SaveOptions{BackupDir: paths.BackupDir(), Keep: 5}, "set "+string(t))
	return err
}

// Remove removes the owned entries of hook type t and reports how many.
func (Adapter) Remove(t hooktypes.Type) (int, error) {
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return 0, err
	}
	n := f.RemoveOwned(t)
	if n > 0 {
		if _, err := save(f, settings.SaveOptions{BackupDir: paths.BackupDir(), Keep: 5}, "remove "+string(t)); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// Uninstall removes every owned entry. The one backup sits next to
// settings.json.
func (Adapter) Uninstall() (removed int, backup string, err error) {
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return 0, "", err
	}
	n := f.RemoveAllOwned()
	if n > 0 {
		o := settings.SaveOptions{BackupDir: filepath.Dir(paths.SettingsPath()), Keep: 1}
		if backup, err = save(f, o, "uninstall"); err != nil {
			return 0, "", err
		}
	}
	return n, backup, nil
}

// save writes f and logs what changed.
func save(f *settings.File, o settings.SaveOptions, what string) (string, error) {
	backup, changed, err := f.Save(o)
	if err != nil {
		return "", err
	}
	if !changed {
		slog.Default().Info("settings unchanged", "path", f.Path, "change", what)
		return backup, nil
	}
	slog.Default().Info("settings written", "path", f.Path, "change", what, "backup", backup)
	return backup, nil
}

// typeOf returns the one hook type all entries belong to.
func typeOf(entries []hooktypes.Entry) (hooktypes.Type, error) {
	if len(entries) == 0 {
		return "", errors.New("no hook entries to install")
	}
	var t hooktypes.Type
	for i, e := range entries {
		if !(Adapter{}).Owns(e.Command) {
			return "", fmt.Errorf("not a Claude hook command: %q", e.Command)
		}
		p, _ := hooktypes.ParseCommand(e.Command)
		if i > 0 && p.Type != t {
			return "", errors.New("hook entries of different types")
		}
		t = p.Type
	}
	return t, nil
}

// ownedMatch reports whether f's owned entries of type t are exactly es.
func ownedMatch(f *settings.File, t hooktypes.Type, es []hooktypes.Entry) bool {
	var have, want []string
	for _, e := range f.Hooks() {
		if e.Owned && e.Parsed.Type == t {
			have = append(have, e.Event+"\x00"+e.Matcher+"\x00"+e.Command)
		}
	}
	for _, e := range es {
		want = append(want, e.Event+"\x00"+e.Matcher+"\x00"+e.Command)
	}
	slices.Sort(have)
	slices.Sort(want)
	return len(want) > 0 && slices.Equal(have, want)
}
