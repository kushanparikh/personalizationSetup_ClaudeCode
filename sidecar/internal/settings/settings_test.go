package settings

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

// userSettings is the user's real settings shape, with the 3 hand-built hooks.
const userSettings = `{
  "attribution": {
    "commit": "",
    "pr": ""
  },
  "model": "opus",
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/git-status.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ]
  },
  "enabledPlugins": {
    "superpowers@claude-plugins-official": true,
    "andrej-karpathy-skills@karpathy-skills": false
  },
  "extraKnownMarketplaces": {
    "karpathy-skills": {
      "source": {
        "source": "github",
        "repo": "forrestchang/andrej-karpathy-skills"
      }
    }
  },
  "tui": "fullscreen",
  "theme": "auto",
  "inputNeededNotifEnabled": true
}
`

const ourStop = "~/.pcs/bin/pcs-hook hook stop --sound Glass --focus"

var finishedDefault = hooktypes.Answers{Sound: "Glass", ClickToFocus: true}

func entries(t *testing.T, typ hooktypes.Type, a hooktypes.Answers) []hooktypes.Entry {
	t.Helper()
	es := hooktypes.Entries(typ, a)
	if es == nil {
		t.Fatalf("Entries(%s, %+v) is nil", typ, a)
	}
	return es
}

// setup writes content to dir/settings.json and returns the path and save
// options using dir/backups.
func setup(t *testing.T, content string) (string, SaveOptions) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, SaveOptions{BackupDir: filepath.Join(dir, "backups"), Keep: 5}
}

func load(t *testing.T, p string) *File {
	t.Helper()
	f, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return f
}

func save(t *testing.T, f *File, o SaveOptions) string {
	t.Helper()
	b, err := f.Save(o)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return b
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetOwnedKeepsEverythingElse(t *testing.T) {
	p, o := setup(t, userSettings)
	f := load(t, p)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	backup := save(t, f, o)

	want := strings.Replace(userSettings, `            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Notification"`, `            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.pcs/bin/pcs-hook hook stop --sound Glass --focus",
            "timeout": 10
          }
        ]
      }
    ],
    "Notification"`, 1)
	if want == userSettings {
		t.Fatal("test bug: replacement did not apply")
	}
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := read(t, backup); got != userSettings {
		t.Fatalf("backup content differs:\n%s", got)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v, want original 0644", st.Mode().Perm())
	}
}

func ownedStopGroups(t *testing.T, f *File) []HookEntry {
	t.Helper()
	var got []HookEntry
	for _, h := range f.Hooks() {
		if h.Owned && h.Parsed.Type == hooktypes.Finished {
			got = append(got, h)
		}
	}
	return got
}

func TestSetOwnedReplacesNotDuplicates(t *testing.T) {
	p, o := setup(t, userSettings)
	f := load(t, p)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	save(t, f, o)

	f = load(t, p)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, hooktypes.Answers{Sound: "Hero", ClickToFocus: false}))
	save(t, f, o)

	f = load(t, p)
	got := ownedStopGroups(t, f)
	if len(got) != 1 {
		t.Fatalf("owned Finished entries = %d, want 1: %+v", len(got), got)
	}
	if want := "~/.pcs/bin/pcs-hook hook stop --sound Hero --no-focus"; got[0].Command != want || got[0].Event != "Stop" {
		t.Fatalf("got %+v, want Stop %q", got[0], want)
	}
	if strings.Count(read(t, p), "pcs-hook") != 1 {
		t.Fatalf("file has duplicate owned commands:\n%s", read(t, p))
	}
}

func TestSetOwnedKeepsPosition(t *testing.T) {
	// Only our hooks exist; re-setting must not move "hooks" or its events.
	in := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.pcs/bin/pcs-hook hook stop --sound Glass --focus",
            "timeout": 10
          }
        ]
      }
    ],
    "PreCompact": []
  },
  "model": "opus"
}
`
	p, o := setup(t, in)
	f := load(t, p)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	save(t, f, o)
	if got := read(t, p); got != in {
		t.Fatalf("got:\n%s\nwant:\n%s", got, in)
	}
}

func TestRemoveOwnedOnlyOurs(t *testing.T) {
	in := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh"
          },
          {
            "type": "command",
            "command": "` + ourStop + `",
            "timeout": 10
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "` + ourStop + `",
            "timeout": 10
          }
        ]
      }
    ],
    "PreCompact": []
  }
}
`
	want := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh"
          }
        ]
      }
    ],
    "PreCompact": []
  }
}
`
	p, o := setup(t, in)
	f := load(t, p)
	if n := f.RemoveOwned(hooktypes.InputNeeded); n != 0 {
		t.Fatalf("RemoveOwned(InputNeeded) = %d, want 0", n)
	}
	if n := f.RemoveOwned(hooktypes.Finished); n != 2 {
		t.Fatalf("RemoveOwned(Finished) = %d, want 2", n)
	}
	save(t, f, o)
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveOwnedDeletesEmptiedEvent(t *testing.T) {
	p, o := setup(t, userSettings)
	f := load(t, p)
	f.SetOwned(hooktypes.InputNeeded, entries(t, hooktypes.InputNeeded, hooktypes.Answers{Sound: "Ping", ClickToFocus: true}))
	save(t, f, o)
	mid := read(t, p)
	for _, k := range []string{`"UserPromptSubmit"`, `"PostToolUse"`, "permission_prompt|elicitation_dialog"} {
		if !strings.Contains(mid, k) {
			t.Fatalf("after SetOwned, missing %s:\n%s", k, mid)
		}
	}

	f = load(t, p)
	if n := f.RemoveOwned(hooktypes.InputNeeded); n != 4 {
		t.Fatalf("RemoveOwned = %d, want 4", n)
	}
	save(t, f, o)
	if got := read(t, p); got != userSettings {
		t.Fatalf("not byte-identical to original:\n%s", got)
	}
}

func TestRemoveAllOwned(t *testing.T) {
	p, o := setup(t, userSettings)
	f := load(t, p)
	f.SetOwned(hooktypes.GitStatus, entries(t, hooktypes.GitStatus, hooktypes.Answers{UntrackedDirty: true}))
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	f.SetOwned(hooktypes.InputNeeded, entries(t, hooktypes.InputNeeded, hooktypes.Answers{Sound: "Ping"}))
	if n := f.RemoveAllOwned(); n != 6 {
		t.Fatalf("RemoveAllOwned = %d, want 6", n)
	}
	save(t, f, o)
	if got := read(t, p); got != userSettings {
		t.Fatalf("not byte-identical to original:\n%s", got)
	}
}

func TestHooksInFileOrder(t *testing.T) {
	in := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "prompt", "prompt": "check"},
          {"type": "command", "command": "` + ourStop + `", "timeout": 10}
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "permission_prompt|elicitation_dialog",
        "hooks": [
          {"type": "command", "command": "~/.pcs/bin/pcs-hook hook notification --sound Ping --no-focus"},
          {"type": "command", "command": "~/.claude/hooks/notify-done.sh"}
        ]
      }
    ]
  }
}`
	p, _ := setup(t, in)
	got := load(t, p).Hooks()
	want := []HookEntry{
		{Event: "Stop"},
		{Event: "Stop", Command: ourStop, Owned: true,
			Parsed: hooktypes.Parsed{Type: hooktypes.Finished, Sub: "stop", Answers: finishedDefault}},
		{Event: "Notification", Matcher: "permission_prompt|elicitation_dialog",
			Command: "~/.pcs/bin/pcs-hook hook notification --sound Ping --no-focus", Owned: true,
			Parsed: hooktypes.Parsed{Type: hooktypes.InputNeeded, Sub: "notification", Answers: hooktypes.Answers{Sound: "Ping"}}},
		{Event: "Notification", Matcher: "permission_prompt|elicitation_dialog", Command: "~/.claude/hooks/notify-done.sh"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestInvalidRefusesWrite(t *testing.T) {
	bad := "{\n  \"a\": 1,\n}"
	p, _ := setup(t, bad)
	_, err := Load(p)
	var ie *InvalidError
	if !errors.As(err, &ie) {
		t.Fatalf("want *InvalidError, got %T %v", err, err)
	}
	if ie.Line != 3 || ie.Path != p {
		t.Fatalf("got %+v, want line 3 path %s", ie, p)
	}
	wantMsg := p + " isn't valid JSON (line 3, column 1): " + ie.Msg + ". Fix it, then reopen. Nothing will be changed until then."
	if ie.Msg == "" || ie.Error() != wantMsg {
		t.Fatalf("Error() = %q, want %q", ie.Error(), wantMsg)
	}

	// Valid at Load, invalid by Save.
	p, o := setup(t, userSettings)
	f := load(t, p)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	if err := os.WriteFile(p, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	backup, err := f.Save(o)
	if !errors.As(err, &ie) {
		t.Fatalf("Save: want *InvalidError, got %T %v", err, err)
	}
	if backup != "" {
		t.Fatalf("backup %q made for an invalid file", backup)
	}
	if got := read(t, p); got != bad {
		t.Fatalf("file changed:\n%s", got)
	}
	if _, err := os.Stat(o.BackupDir); !os.IsNotExist(err) {
		t.Fatalf("backup dir touched: %v", err)
	}
	ents, _ := os.ReadDir(filepath.Dir(p))
	if len(ents) != 1 {
		t.Fatalf("stray files left: %v", ents)
	}
}

func TestMalformedHooksUntouched(t *testing.T) {
	// SetOwned is a no-op where the shape it would edit is malformed.
	for name, in := range map[string]string{
		"hooks not object": "{\n  \"hooks\": \"x\"\n}\n",
		"event not array":  "{\n  \"hooks\": {\n    \"Stop\": {}\n  }\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			p, o := setup(t, in)
			f := load(t, p)
			f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
			save(t, f, o)
			if got := read(t, p); got != in {
				t.Fatalf("got:\n%s\nwant:\n%s", got, in)
			}
		})
	}
	// Malformed groups and inner lists are skipped but kept as they are.
	for name, in := range map[string]string{
		"group not object":     "{\n  \"hooks\": {\n    \"Stop\": [\n      \"x\"\n    ]\n  }\n}\n",
		"group hooks not list": "{\n  \"hooks\": {\n    \"Stop\": [\n      {\n        \"hooks\": \"x\"\n      }\n    ]\n  }\n}\n",
		"inner not object":     "{\n  \"hooks\": {\n    \"Stop\": [\n      {\n        \"hooks\": [\n          7\n        ]\n      }\n    ]\n  }\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			p, o := setup(t, in)
			f := load(t, p)
			f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
			if n := f.RemoveAllOwned(); n != 1 {
				t.Fatalf("RemoveAllOwned = %d, want 1", n)
			}
			save(t, f, o)
			if got := read(t, p); got != in {
				t.Fatalf("got:\n%s\nwant:\n%s", got, in)
			}
		})
	}
}

func TestSaveFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "settings.json")
	link := filepath.Join(dir, ".claude", "settings.json")
	for _, d := range []string{filepath.Dir(target), filepath.Dir(link)} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(target, []byte(userSettings), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../dotfiles/settings.json", link); err != nil {
		t.Fatal(err)
	}
	f := load(t, link)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	save(t, f, SaveOptions{BackupDir: filepath.Join(dir, "backups"), Keep: 5})

	st, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("settings.json is no longer a symlink: %v", st.Mode())
	}
	if !strings.Contains(read(t, target), ourStop) {
		t.Fatalf("target lacks new content:\n%s", read(t, target))
	}
	ents, _ := os.ReadDir(filepath.Dir(link))
	if len(ents) != 1 {
		t.Fatalf("stray files next to link: %v", ents)
	}
}

func TestBackupsPruned(t *testing.T) {
	p, o := setup(t, userSettings)
	if err := os.MkdirAll(o.BackupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	others := []string{"notes.txt", "settings.pcs-backup-foo.json", "settings.json"}
	for _, n := range others {
		if err := os.WriteFile(filepath.Join(o.BackupDir, n), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var pre []string // file content before each save
	for i := 0; i < 7; i++ {
		pre = append(pre, read(t, p))
		f := load(t, p)
		f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, hooktypes.Answers{Sound: hooktypes.Sounds[i]}))
		save(t, f, o)
	}

	pat := regexp.MustCompile(`^settings\.pcs-backup-\d{8}T\d{6}\.\d{3}Z(-\d+)?\.json$`)
	var kept []string
	ents, err := os.ReadDir(o.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if pat.MatchString(e.Name()) {
			kept = append(kept, read(t, filepath.Join(o.BackupDir, e.Name())))
		}
	}
	if len(kept) != 5 {
		t.Fatalf("backups = %d, want 5", len(kept))
	}
	for _, want := range pre[2:] {
		if !slices.Contains(kept, want) {
			t.Fatalf("a newest backup was pruned; missing:\n%s", want)
		}
	}
	for _, n := range others {
		if got := read(t, filepath.Join(o.BackupDir, n)); got != "keep" {
			t.Fatalf("%s changed: %q", n, got)
		}
	}
}

func TestMissingFileCreated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := paths.SettingsPath()
	if _, err := os.Stat(filepath.Dir(p)); !os.IsNotExist(err) {
		t.Fatalf("~/.claude exists: %v", err)
	}
	f := load(t, p)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	if b := save(t, f, SaveOptions{BackupDir: paths.BackupDir(), Keep: 5}); b != "" {
		t.Fatalf("backup %q for a missing file", b)
	}
	want := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.pcs/bin/pcs-hook hook stop --sound Glass --focus",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
`
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s", got)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v, want 0600", st.Mode().Perm())
	}
	dst, err := os.Stat(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if dst.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v, want 0700", dst.Mode().Perm())
	}
}

func TestEmptyFileIsEmptyObject(t *testing.T) {
	p, o := setup(t, "")
	f := load(t, p)
	if len(f.Hooks()) != 0 {
		t.Fatal("hooks in empty file")
	}
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	if b := save(t, f, o); b != "" {
		t.Fatalf("backup %q for an empty file", b)
	}
	if !strings.Contains(read(t, p), ourStop) {
		t.Fatalf("not written:\n%s", read(t, p))
	}
}

func TestPruneOrderWithSuffixes(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"settings.pcs-backup-20261024T101010.000Z.json",
		"settings.pcs-backup-20261024T101010.123Z.json",
		"settings.pcs-backup-20261024T101010.123Z-1.json",
		"settings.pcs-backup-20261024T101010.123Z-2.json",
		"settings.pcs-backup-20261024T101010.123Z-10.json",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := prune(dir, 2); err != nil {
		t.Fatal(err)
	}
	var got []string
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		got = append(got, e.Name())
	}
	want := []string{names[4], names[3]} // ReadDir sorts by name: "-10" < "-2"
	if !slices.Equal(got, want) {
		t.Fatalf("kept %v, want %v", got, want)
	}
}

func backupNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if backupName.MatchString(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestIdenticalSavesKeepFirstBackup(t *testing.T) {
	p, o := setup(t, userSettings)
	for i := 0; i < 5; i++ {
		f := load(t, p)
		f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
		b := save(t, f, o)
		if i > 0 && b != "" {
			t.Fatalf("save %d made backup %q for an identical file", i, b)
		}
	}
	names := backupNames(t, o.BackupDir)
	if len(names) != 1 {
		t.Fatalf("backups = %v, want exactly 1", names)
	}
	if got := read(t, filepath.Join(o.BackupDir, names[0])); got != userSettings {
		t.Fatalf("backup isn't the original:\n%s", got)
	}
	if !strings.Contains(read(t, p), ourStop) {
		t.Fatalf("not written:\n%s", read(t, p))
	}
}

func TestSaveRefusesIfChangedSinceLoad(t *testing.T) {
	const changed = "{\n  \"model\": \"sonnet\"\n}\n"
	t.Run("edited", func(t *testing.T) {
		p, o := setup(t, userSettings)
		f := load(t, p)
		if err := os.WriteFile(p, []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
		_, err := f.Save(o)
		if !errors.Is(err, ErrChangedSinceLoad) {
			t.Fatalf("Save err = %v, want ErrChangedSinceLoad", err)
		}
		if err.Error() != "~/.claude/settings.json changed while this app was editing it. Nothing was changed; try again." {
			t.Fatalf("message = %q", err.Error())
		}
		if got := read(t, p); got != changed {
			t.Fatalf("file was written:\n%s", got)
		}
		if n := backupNames(t, o.BackupDir); len(n) != 0 {
			t.Fatalf("backups made: %v", n)
		}
	})
	t.Run("created", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "settings.json")
		f := load(t, p)
		if err := os.WriteFile(p, []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
		if _, err := f.Save(SaveOptions{BackupDir: filepath.Join(dir, "backups")}); !errors.Is(err, ErrChangedSinceLoad) {
			t.Fatalf("Save err = %v, want ErrChangedSinceLoad", err)
		}
		if got := read(t, p); got != changed {
			t.Fatalf("file was written:\n%s", got)
		}
	})
	t.Run("deleted", func(t *testing.T) {
		p, o := setup(t, "")
		f := load(t, p)
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
		if _, err := f.Save(o); !errors.Is(err, ErrChangedSinceLoad) {
			t.Fatalf("Save err = %v, want ErrChangedSinceLoad", err)
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("file was written: %v", err)
		}
	})
}

func TestSaveRefusesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "settings.json")
	target := filepath.Join(dir, "missing", "settings.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	f := load(t, link)
	f.SetOwned(hooktypes.Finished, entries(t, hooktypes.Finished, finishedDefault))
	if _, err := f.Save(SaveOptions{BackupDir: filepath.Join(dir, "backups")}); err == nil {
		t.Fatal("Save through a dangling symlink succeeded")
	}
	st, err := os.Lstat(link)
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v %v", st, err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("target created: %v", err)
	}
}

func TestRemoveAllOwnedByPrefix(t *testing.T) {
	const in = `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.pcs/bin/pcs-hook hook future-thing --x",
            "timeout": 5
          },
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
`
	p, o := setup(t, in)
	f := load(t, p)
	if n := f.RemoveOwned(hooktypes.Finished); n != 0 {
		t.Fatalf("RemoveOwned(finished) = %d, want 0", n)
	}
	if n := f.RemoveAllOwned(); n != 1 {
		t.Fatalf("RemoveAllOwned = %d, want 1", n)
	}
	save(t, f, o)
	got := read(t, p)
	if strings.Contains(got, "future-thing") || !strings.Contains(got, "notify-done.sh") {
		t.Fatalf("got:\n%s", got)
	}
}
