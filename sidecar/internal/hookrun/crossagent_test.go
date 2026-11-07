package hookrun

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/agent"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/agent/claude"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/ipc"
)

// fakeAdapter is a second agent that reuses Claude's session ids, so the
// test can check that alerts stay apart.
type fakeAdapter struct{}

func (fakeAdapter) DisplayName() string     { return "Fake Agent" }
func (fakeAdapter) Detect() agent.Detection { return agent.Detection{} }
func (fakeAdapter) Capabilities() []string {
	return []string{agent.TaskDone, agent.NeedsInput, agent.MovedOn}
}
func (fakeAdapter) Install([]hooktypes.Entry) error { return nil }
func (fakeAdapter) Uninstall() (int, string, error) { return 0, "", nil }
func (fakeAdapter) Owns(string) bool                { return false }
func (f fakeAdapter) Parse(sub string, data []byte) agent.Event {
	var in struct {
		ID      string `json:"session_id"`
		Cwd     string `json:"cwd"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &in)
	e := agent.Event{Schema: agent.Schema, Agent: "fake", AgentName: f.DisplayName(), SessionID: in.ID, Cwd: in.Cwd, Message: in.Message}
	switch sub {
	case "stop":
		e.Event = agent.TaskDone
	case "notification":
		e.Event = agent.NeedsInput
	case "moved-on":
		e.Event = agent.MovedOn
	}
	return e
}

// board models how the app tracks panels: open on panel, close on a
// moved_on with the same sessionId (app/src-tauri/src/board.rs).
type board map[string]bool

func (b board) apply(e ipc.Event) {
	switch e.Kind {
	case ipc.KindPanel:
		b[e.SessionID] = true
	case ipc.KindMovedOn:
		delete(b, e.SessionID)
	}
}

func TestCrossAgentTitlesAndKeys(t *testing.T) {
	r := &rec{}
	fire := func(ad agent.Adapter, sub, in string) ipc.Event {
		t.Helper()
		n := len(r.events)
		if code := Run(ad, sub, []string{"--sound", "none", "--no-focus"}, strings.NewReader(in), &bytes.Buffer{}, r.deps(nil)); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if len(r.events) != n+1 {
			t.Fatalf("%s: %d events sent", sub, len(r.events)-n)
		}
		return r.events[n]
	}
	const payload = `{"session_id":"s1","cwd":"/x/r","message":"approve?"}`
	cl, fk := claude.Adapter{}, fakeAdapter{}

	for _, c := range []struct {
		ad         agent.Adapter
		title, key string
	}{{cl, "Claude Code — r", "claude:s1"}, {fk, "Fake Agent — r", "fake:s1"}} {
		if e := fire(c.ad, "stop", payload); e.Title != c.title || e.SessionID != c.key {
			t.Errorf("toast: %+v, want title %q key %q", e, c.title, c.key)
		}
		if e := fire(c.ad, "notification", payload); e.Title != c.title || e.SessionID != c.key || e.Message != "approve?" {
			t.Errorf("panel: %+v, want title %q key %q", e, c.title, c.key)
		}
	}

	b := board{}
	for _, e := range r.events {
		b.apply(e)
	}
	if !b["claude:s1"] || !b["fake:s1"] {
		t.Fatalf("both panels should be open: %v", b)
	}
	b.apply(fire(fk, "moved-on", `{"session_id":"s1"}`))
	if !b["claude:s1"] || b["fake:s1"] {
		t.Fatalf("fake's moved_on must close only fake's panel: %v", b)
	}
	b.apply(fire(cl, "moved-on", `{"session_id":"s1","hook_event_name":"UserPromptSubmit"}`))
	if len(b) != 0 {
		t.Fatalf("claude's moved_on should close claude's panel: %v", b)
	}
}
