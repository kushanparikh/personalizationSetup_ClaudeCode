package report

import (
	"bytes"
	"encoding/json"
	"flag"
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
	have := map[perms.ID]perms.Status{perms.AutomationTerminal: perms.NotAsked}

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

func TestFinishedAnswersFromStopEntry(t *testing.T) {
	// The revisit entries come first in file order and carry no answers;
	// the reported answers must still be the stop entry's.
	f, err := load(t, `{
  "hooks": {
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "~/.pcs/bin/pcs-hook hook revisit", "timeout": 5}]}],
    "SessionEnd": [{"hooks": [{"type": "command", "command": "~/.pcs/bin/pcs-hook hook revisit", "timeout": 5}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "~/.pcs/bin/pcs-hook hook stop --sound Hero --no-focus", "timeout": 10}]}]
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	st := emptyStore()
	st.Hooks[hooktypes.Finished] = hooktypes.Answers{Sound: "Glass", ClickToFocus: true}
	want := hooktypes.Answers{Sound: "Hero", ClickToFocus: false}

	if a, enabled := CurrentAnswers(f, st, hooktypes.Finished); !enabled || a != want {
		t.Fatalf("CurrentAnswers = %+v enabled=%v, want %+v", a, enabled, want)
	}
	if h := hookView(t, Build(f, nil, st, nil), hooktypes.Finished); !h.Enabled || h.Answers != want {
		t.Fatalf("finished view = %+v", h)
	}
}

func TestFinishedRevisitOnlyUsesStoredAnswers(t *testing.T) {
	f, err := load(t, `{"hooks": {"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "~/.pcs/bin/pcs-hook hook revisit"}]}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	st := emptyStore()
	saved := hooktypes.Answers{Sound: "Tink", ClickToFocus: true}
	st.Hooks[hooktypes.Finished] = saved
	if a, enabled := CurrentAnswers(f, st, hooktypes.Finished); !enabled || a != saved {
		t.Fatalf("CurrentAnswers = %+v enabled=%v, want %+v", a, enabled, saved)
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
	r := Build(f, nil, emptyStore(), map[perms.ID]perms.Status{perms.AutomationTerminal: perms.NotAsked})
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, k := range []string{`"hooks":`, `"available":`, `"otherHooks":`, `"pending":`, `"settingsError":`, `"logPath":`,
		`"needs":`, `"enabled":`, `"answers":`, `"id":"automation-terminal"`, `"settingsUrl":`, `"affects":["Finished working"]`, `"title":"Control Terminal"`} {
		if !strings.Contains(s, k) {
			t.Errorf("missing %s in %s", k, s)
		}
	}
	if !strings.Contains(s, "Privacy & Security") || strings.Contains(s, "\\"+"u0026") {
		t.Errorf("want a literal & in the output: %s", s)
	}
}

var update = flag.Bool("update", false, "rewrite testdata golden files")

const goldenSettings = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo one"}]}
    ]
  }
}
`

// TestReportGolden pins the JSON the app reads. The TS tests read
// this file (Rust reads serve-lines.golden.jsonl), so a shape change here fails them until the golden is
// regenerated (go test ./cmd/pcs ./internal/report -update) and the consumers are fixed.
func TestReportGolden(t *testing.T) {
	f, err := load(t, goldenSettings)
	if err != nil {
		t.Fatal(err)
	}
	setOwned(t, f, hooktypes.GitStatus, hooktypes.Answers{UntrackedDirty: true, Sound: "Glass", ClickToFocus: true})
	setOwned(t, f, hooktypes.Finished, hooktypes.Answers{Sound: "Glass", ClickToFocus: true})
	st := emptyStore()
	// Stored but not enabled: status "off".
	st.Hooks[hooktypes.InputNeeded] = hooktypes.Answers{Sound: "Ping", ClickToFocus: true}
	have := map[perms.ID]perms.Status{perms.AutomationTerminal: perms.NotAsked}

	r := Build(f, nil, st, have)
	// Only three hook types exist and each status needs its own, so no type is
	// left over for "available". Add one real def to pin that shape too.
	d, _ := hooktypes.Get(hooktypes.InputNeeded)
	r.Available = append(r.Available, d)
	r.LogPath = "/tmp/pcs.log" // machine-specific otherwise

	seen := map[string]bool{}
	for _, h := range r.Hooks {
		seen[h.Status] = true
	}
	if !seen[StatusOK] || !seen[StatusPending] || !seen[StatusOff] || len(r.OtherHooks) != 1 || len(r.Pending) != 1 {
		t.Fatalf("golden input does not cover every shape: %+v", r)
	}

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "report.golden.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, b.Bytes()) {
		t.Fatalf("report JSON differs from %s (run go test ./cmd/pcs ./internal/report -update)\ngot:\n%s\nwant:\n%s", golden, b.Bytes(), want)
	}
}
