package claude

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/agent"
)

func payload(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParsePayloads(t *testing.T) {
	base := agent.Event{Schema: 1, Agent: "claude", AgentName: "Claude Code", SessionID: "s1", Cwd: "/x/My Project"}
	with := func(f func(*agent.Event)) agent.Event { e := base; f(&e); return e }
	cases := []struct {
		sub, file string
		want      agent.Event
	}{
		{"session-start", "session-start.json", with(func(e *agent.Event) { e.Event = agent.SessionStart })},
		{"stop", "stop.json", with(func(e *agent.Event) { e.Event = agent.TaskDone })},
		{"notification", "notification.json", with(func(e *agent.Event) {
			e.Event, e.Message = agent.NeedsInput, "Claude needs your permission to use Bash"
		})},
		{"moved-on", "user-prompt-submit.json", with(func(e *agent.Event) { e.Event, e.Cause = agent.MovedOn, "UserPromptSubmit" })},
		{"moved-on", "post-tool-use.json", with(func(e *agent.Event) { e.Event, e.Cause = agent.MovedOn, "PostToolUse" })},
		{"moved-on", "stop.json", with(func(e *agent.Event) { e.Event, e.Cause = agent.MovedOn, "Stop" })},
		{"revisit", "user-prompt-submit.json", with(func(e *agent.Event) { e.Event, e.Cause = agent.MovedOn, "UserPromptSubmit" })},
		{"revisit", "session-end.json", with(func(e *agent.Event) { e.Event, e.Cause = agent.SessionEnd, "SessionEnd" })},
	}
	for _, c := range cases {
		if got := (Adapter{}).Parse(c.sub, payload(t, c.file)); got != c.want {
			t.Errorf("%s %s:\n got %+v\nwant %+v", c.sub, c.file, got, c.want)
		}
	}
}

func TestParseBadInputStillUsable(t *testing.T) {
	for _, in := range []string{"", "not json", `{"session_id":5}`} {
		e := (Adapter{}).Parse("notification", []byte(in))
		want := agent.Event{Schema: 1, Agent: "claude", AgentName: "Claude Code", Event: agent.NeedsInput, Message: "Claude needs your input"}
		if e != want {
			t.Errorf("%q: %+v", in, e)
		}
	}
	if e := (Adapter{}).Parse("bogus", []byte(`{}`)); e.Event != "" {
		t.Errorf("unknown sub gave event %q", e.Event)
	}
}

func TestOwns(t *testing.T) {
	cases := map[string]bool{
		"~/.pcs/bin/pcs-hook hook stop --sound Glass --focus":              true,
		"~/.pcs/bin/pcs-hook hook --agent claude moved-on":                 true,
		"~/.pcs/bin/pcs-hook hook --agent=claude revisit":                  true,
		"~/.pcs/bin/pcs-hook hook --agent kimi stop --sound Glass --focus": false,
		"~/.pcs/bin/pcs-hook hook --agent":                                 false,
		"~/.claude/hooks/notify-done.sh":                                   false,
		"~/.pcs/bin/pcs-hook hook bogus":                                   false,
	}
	for cmd, want := range cases {
		if got := (Adapter{}).Owns(cmd); got != want {
			t.Errorf("Owns(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestDetect(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	d := (Adapter{}).Detect()
	if d.Installed || d.ConfigPath != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("empty home: %+v", d)
	}
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if d := (Adapter{}).Detect(); !d.Installed {
		t.Fatalf("with ~/.claude: %+v", d)
	}
}

func TestCapabilitiesCoverEveryEvent(t *testing.T) {
	got := (Adapter{}).Capabilities()
	for _, e := range []string{agent.SessionStart, agent.TaskDone, agent.NeedsInput, agent.MovedOn, agent.SessionEnd} {
		if !slices.Contains(got, e) {
			t.Errorf("missing %s", e)
		}
	}
}
