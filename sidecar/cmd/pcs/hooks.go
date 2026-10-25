package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/launcher"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/perms"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/report"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/settings"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/state"
)

const terminalBundleID = "com.apple.Terminal"

// hookFlags are the flags shared by the commands that change one hook type
// and print the fresh report.
type hookFlags struct {
	fs  *flag.FlagSet
	typ *string
}

func newHookFlags(name string) hookFlags {
	fs := newFlags(name)
	return hookFlags{fs: fs, typ: fs.String("type", "", "hook type")}
}

// parse returns the hook type's Def.
func (h hookFlags) parse(args []string) (hooktypes.Def, error) {
	if err := parse(h.fs, args); err != nil {
		return hooktypes.Def{}, err
	}
	d, ok := hooktypes.Get(hooktypes.Type(*h.typ))
	if !ok {
		return hooktypes.Def{}, fmt.Errorf("unknown hook type %q", *h.typ)
	}
	return d, nil
}

// installLauncher always (re)writes the launcher for bin; used by install-launcher.
func installLauncher(bin string) (bool, error) {
	return logLauncher(bin, launcher.Install)
}

// ensureLauncher writes the launcher only when it's missing, so write commands
// never clobber a launcher set up with install-launcher --bin.
func ensureLauncher() error {
	_, err := logLauncher(paths.AppBinary(), launcher.EnsureInstalled)
	return err
}

func logLauncher(bin string, install func(path, bin string) (bool, error)) (bool, error) {
	changed, err := install(paths.LauncherPath(), bin)
	if err != nil {
		return false, fmt.Errorf("couldn't install the hook launcher: %w", err)
	}
	if changed {
		slog.Default().Info("launcher installed", "path", paths.LauncherPath(), "bin", bin)
	}
	return changed, nil
}

func loadState() *state.Store {
	st, err := state.Load(paths.StatePath())
	if err != nil {
		slog.Default().Warn("state unreadable, starting empty", "err", err)
	}
	return st
}

func saveState(st *state.Store) {
	if err := st.Save(paths.StatePath()); err != nil {
		slog.Default().Error("state save failed", "err", err)
	}
}

func saveSettings(f *settings.File, o settings.SaveOptions, what string) (string, error) {
	backup, err := f.Save(o)
	if err != nil {
		return "", err
	}
	slog.Default().Info("settings written", "path", f.Path, "change", what, "backup", backup)
	return backup, nil
}

// buildReport reads everything fresh and checks the Automation permission.
func buildReport() report.Report {
	f, err := settings.Load(paths.SettingsPath())
	auto := perms.CheckAutomation(terminalBundleID)
	slog.Default().Info("permission check", "id", perms.AutomationTerminal, "status", auto)
	return report.Build(f, err, loadState(), map[perms.ID]perms.Status{
		perms.AutomationTerminal: auto,
	})
}

// setHook writes the owned entries of d's type with answers a.
func setHook(d hooktypes.Def, a hooktypes.Answers) error {
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return err
	}
	es := hooktypes.Entries(d.Type, a)
	f.SetOwned(d.Type, es)
	if !ownedMatch(f, d.Type, es) { // SetOwned refused a malformed shape
		return fmt.Errorf(`~/.claude/settings.json has a "hooks" section this app can't safely edit, so %s wasn't added. Nothing was changed.`, d.Name)
	}
	_, err = saveSettings(f, settings.SaveOptions{BackupDir: paths.BackupDir(), Keep: 5}, "set "+string(d.Type))
	return err
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

func cmdState(args []string) (any, error) {
	fs := newFlags("state")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	return buildReport(), nil
}

func cmdApply(args []string) (any, error) {
	h := newHookFlags("apply")
	answersIn := h.fs.String("answers", "", "answers JSON")
	d, err := h.parse(args)
	if err != nil {
		return nil, err
	}
	a := d.Defaults
	if *answersIn != "" {
		if err := decodeAnswers(*answersIn, &a); err != nil {
			return nil, fmt.Errorf("invalid --answers: %w", err)
		}
	}
	if err := hooktypes.Validate(d.Type, a); err != nil {
		return nil, err
	}
	if err := ensureLauncher(); err != nil {
		return nil, err
	}
	if err := setHook(d, a); err != nil {
		return nil, err
	}
	st := loadState()
	st.Hooks[d.Type] = a
	saveState(st)
	return buildReport(), nil
}

// decodeAnswers decodes one JSON object into a, rejecting unknown keys and
// trailing data.
func decodeAnswers(in string, a *hooktypes.Answers) error {
	dec := json.NewDecoder(strings.NewReader(in))
	dec.DisallowUnknownFields()
	if err := dec.Decode(a); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("unexpected data after the answers object")
	}
	return nil
}

func cmdEnable(args []string) (any, error) {
	d, err := newHookFlags("enable").parse(args)
	if err != nil {
		return nil, err
	}
	if err := ensureLauncher(); err != nil {
		return nil, err
	}
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return nil, err
	}
	st := loadState()
	a, _ := report.CurrentAnswers(f, st, d.Type)
	if err := hooktypes.Validate(d.Type, a); err != nil {
		return nil, fmt.Errorf("the saved answers for %s are invalid (%w); set it up again", d.Name, err)
	}
	if err := setHook(d, a); err != nil {
		return nil, err
	}
	st.Hooks[d.Type] = a
	saveState(st)
	return buildReport(), nil
}

func cmdDisable(args []string) (any, error) {
	d, err := newHookFlags("disable").parse(args)
	if err != nil {
		return nil, err
	}
	if err := ensureLauncher(); err != nil {
		return nil, err
	}
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return nil, err
	}
	st := loadState()
	if a, enabled := report.CurrentAnswers(f, st, d.Type); enabled {
		st.Hooks[d.Type] = a // keep the answers so enable restores them
		saveState(st)
		f.RemoveOwned(d.Type)
		if _, err := saveSettings(f, settings.SaveOptions{BackupDir: paths.BackupDir(), Keep: 5}, "disable "+string(d.Type)); err != nil {
			return nil, err
		}
	}
	return buildReport(), nil
}

func cmdRemove(args []string) (any, error) {
	d, err := newHookFlags("remove").parse(args)
	if err != nil {
		return nil, err
	}
	if err := ensureLauncher(); err != nil {
		return nil, err
	}
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return nil, err
	}
	if f.RemoveOwned(d.Type) > 0 {
		if _, err := saveSettings(f, settings.SaveOptions{BackupDir: paths.BackupDir(), Keep: 5}, "remove "+string(d.Type)); err != nil {
			return nil, err
		}
	}
	st := loadState()
	if _, ok := st.Hooks[d.Type]; ok {
		delete(st.Hooks, d.Type)
		saveState(st)
	}
	return buildReport(), nil
}

func cmdUninstall(args []string) (any, error) {
	fs := newFlags("uninstall")
	yes := fs.Bool("yes", false, "confirm")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	if !*yes {
		return nil, errors.New("uninstall removes the app's hooks and data; run it with --yes to confirm")
	}
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		return nil, err
	}
	n := f.RemoveAllOwned()
	backup := ""
	if n > 0 {
		o := settings.SaveOptions{BackupDir: filepath.Dir(paths.SettingsPath()), Keep: 1}
		if backup, err = saveSettings(f, o, "uninstall"); err != nil {
			return nil, err
		}
	}
	slog.Default().Info("uninstall: removing app data", "removedHooks", n, "pcsDir", paths.PCSDir(), "logDir", paths.LogDir())
	for _, dir := range []string{paths.PCSDir(), paths.LogDir()} {
		if err := os.RemoveAll(dir); err != nil {
			return nil, fmt.Errorf("hooks were removed, but %s couldn't be deleted: %w", dir, err)
		}
	}
	return map[string]any{"removedHooks": n, "backup": backup}, nil
}
