package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/perms"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/settings"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/state"
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
    "superpowers@claude-plugins-official": true
  },
  "tui": "fullscreen",
  "theme": "auto"
}
`

func load(t *testing.T, content string) (*settings.File, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	p := paths.SettingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return settings.Load(p)
}

func emptyStore() *state.Store {
	return &state.Store{Version: 1, Hooks: map[hooktypes.Type]hooktypes.Answers{}}
}

func setOwned(t *testing.T, f *settings.File, typ hooktypes.Type, a hooktypes.Answers) {
	t.Helper()
	es := hooktypes.Entries(typ, a)
	if es == nil {
		t.Fatalf("no entries for %s %+v", typ, a)
	}
	f.SetOwned(typ, es)
}

func hookView(t *testing.T, r Report, typ hooktypes.Type) HookView {
	t.Helper()
	for _, h := range r.Hooks {
		if h.Type == typ {
			return h
		}
	}
	t.Fatalf("hook %s not in report: %+v", typ, r.Hooks)
	return HookView{}
}

func TestStatusPendingUntilGranted(t *testing.T) {
	f, err := load(t, userSettings)
	if err != nil {
		t.Fatal(err)
	}
	a := hooktypes.Answers{Sound: "Glass", ClickToFocus: true}
	setOwned(t, f, hooktypes.Finished, a)
	have := map[perms.ID]perms.Status{perms.Notifications: perms.Granted, perms.AutomationTerminal: perms.NotAsked}

	r := Build(f, nil, emptyStore(), have)
	h := hookView(t, r, hooktypes.Finished)
	if !h.Enabled || h.Status != "pending" || h.Answers != a || h.Name != "Finished working" {
		t.Fatalf("hook view = %+v", h)
	}
	if len(r.Pending) != 1 {
		t.Fatalf("pending = %+v", r.Pending)
	}
	p := r.Pending[0]
	if p.ID != perms.AutomationTerminal || p.Status != perms.NotAsked || !slices.Equal(p.Affects, []string{"Finished working"}) || p.Title == "" {
		t.Fatalf("pending item = %+v", p)
	}

	have[perms.AutomationTerminal] = perms.Granted
	r = Build(f, nil, emptyStore(), have)
	if h := hookView(t, r, hooktypes.Finished); h.Status != "ok" {
		t.Fatalf("all granted: status = %q", h.Status)
	}
	if len(r.Pending) != 0 {
		t.Fatalf("all granted: pending = %+v", r.Pending)
	}
}

func TestOffHookNotInPending(t *testing.T) {
	f, err := load(t, userSettings)
	if err != nil {
		t.Fatal(err)
	}
	st := emptyStore()
	saved := hooktypes.Answers{Sound: "Hero", ClickToFocus: true}
	st.Hooks[hooktypes.Finished] = saved

	r := Build(f, nil, st, map[perms.ID]perms.Status{})
	h := hookView(t, r, hooktypes.Finished)
	if h.Enabled || h.Status != "off" || h.Answers != saved {
		t.Fatalf("off hook view = %+v", h)
	}
	if len(r.Pending) != 0 {
		t.Fatalf("pending = %+v", r.Pending)
	}
	for _, d := range r.Available {
		if d.Type == hooktypes.Finished {
			t.Fatalf("finished is in Hooks, so not Available: %+v", r.Available)
		}
	}
}

func TestInputNeededAnswersFromNotificationEntry(t *testing.T) {
	f, err := load(t, userSettings)
	if err != nil {
		t.Fatal(err)
	}
	a := hooktypes.Answers{Sound: "Submarine", ClickToFocus: false}
	setOwned(t, f, hooktypes.InputNeeded, a)
	st := emptyStore()
	st.Hooks[hooktypes.InputNeeded] = hooktypes.Answers{Sound: "Ping", ClickToFocus: true}

	h := hookView(t, Build(f, nil, st, nil), hooktypes.InputNeeded)
	if !h.Enabled || h.Answers != a || h.Status != "ok" || len(h.Needs) != 0 {
		t.Fatalf("input-needed view = %+v", h)
	}
}

func TestOtherHooksListed(t *testing.T) {
	f, err := load(t, userSettings)
	if err != nil {
		t.Fatal(err)
	}
	r := Build(f, nil, emptyStore(), nil)
	want := []OtherHook{
		{Event: "SessionStart", Command: "~/.claude/hooks/git-status.sh"},
		{Event: "Stop", Command: "~/.claude/hooks/notify-done.sh"},
		{Event: "Notification", Command: "~/.claude/hooks/notify-done.sh"},
	}
	if !slices.Equal(r.OtherHooks, want) {
		t.Fatalf("other hooks = %+v", r.OtherHooks)
	}
	if len(r.Hooks) != 0 {
		t.Fatalf("hooks = %+v", r.Hooks)
	}
	if len(r.Available) != len(hooktypes.All()) {
		t.Fatalf("available = %+v", r.Available)
	}
	if r.LogPath != paths.LogPath() || r.SettingsError != "" {
		t.Fatalf("logPath %q settingsError %q", r.LogPath, r.SettingsError)
	}
}

func TestInvalidSettingsOnlyError(t *testing.T) {
	_, err := load(t, `{"hooks": `)
	if err == nil {
		t.Fatal("want load error")
	}
	st := emptyStore()
	st.Hooks[hooktypes.Finished] = hooktypes.Answers{Sound: "Glass"}
	r := Build(nil, err, st, nil)
	if r.SettingsError != err.Error() || r.LogPath != paths.LogPath() {
		t.Fatalf("report = %+v", r)
	}
	if len(r.Hooks)+len(r.Available)+len(r.OtherHooks)+len(r.Pending) != 0 {
		t.Fatalf("only error expected: %+v", r)
	}
}

func TestJSONIsCamelCase(t *testing.T) {
	f, err := load(t, userSettings)
	if err != nil {
		t.Fatal(err)
	}
	setOwned(t, f, hooktypes.Finished, hooktypes.Answers{Sound: "Glass", ClickToFocus: true})
	r := Build(f, nil, emptyStore(), map[perms.ID]perms.Status{perms.Notifications: perms.Granted})
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, k := range []string{`"hooks":`, `"available":`, `"otherHooks":`, `"pending":`, `"settingsError":`, `"logPath":`,
		`"needs":`, `"enabled":`, `"answers":`, `"id":"automation-terminal"`, `"settingsUrl":`, `"affects":["Finished working"]`, `"title":"Control Terminal"`} {
		if !strings.Contains(s, k) {
			t.Errorf("missing %s in %s", k, s)
		}
	}
	if strings.Contains(s, `&`) {
		t.Errorf("HTML-escaped output: %s", s)
	}
}
