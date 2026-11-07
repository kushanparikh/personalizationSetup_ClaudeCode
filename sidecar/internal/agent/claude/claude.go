// Package claude is the Claude Code adapter: its hook payloads, and the
// entries this app writes into ~/.claude/settings.json.
package claude

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/agent"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

// Name is the adapter's id, used by `pcs hook --agent` and in session keys.
const Name = "claude"

const defaultPrompt = "Claude needs your input"

// Adapter is the Claude Code adapter.
type Adapter struct{}

func (Adapter) DisplayName() string { return "Claude Code" }

// Detect reports Claude Code as installed when `claude` is on PATH or
// ~/.claude exists.
func (Adapter) Detect() agent.Detection {
	d := agent.Detection{ConfigPath: paths.SettingsPath()}
	if _, err := exec.LookPath("claude"); err == nil {
		d.Installed = true
	} else if fi, err := os.Stat(filepath.Dir(d.ConfigPath)); err == nil && fi.IsDir() {
		d.Installed = true
	}
	return d
}

func (Adapter) Capabilities() []string {
	return []string{agent.SessionStart, agent.TaskDone, agent.NeedsInput, agent.MovedOn, agent.SessionEnd}
}

// input is the subset of Claude Code's hook JSON the app uses.
type input struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Event     string `json:"hook_event_name"`
	Message   string `json:"message"`
}

// Parse maps a hook subcommand and its payload to the canonical event. The
// subcommand decides the event: Claude's Stop runs both `stop` (task done)
// and `moved-on` (close the panel).
func (a Adapter) Parse(sub string, data []byte) agent.Event {
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		in = input{}
	}
	e := agent.Event{Schema: agent.Schema, Agent: Name, AgentName: a.DisplayName(),
		SessionID: in.SessionID, Cwd: in.Cwd}
	switch sub {
	case "session-start":
		e.Event = agent.SessionStart
	case "stop":
		e.Event = agent.TaskDone
	case "notification":
		e.Event = agent.NeedsInput
		e.Message = in.Message
		if e.Message == "" {
			e.Message = defaultPrompt
		}
	case "moved-on", "revisit":
		e.Event = agent.MovedOn
		if in.Event == "SessionEnd" {
			e.Event = agent.SessionEnd
		}
		e.Cause = in.Event
	}
	return e
}

// Owns reports whether command is one of this app's Claude hook commands.
// Commands without --agent predate adapters and are Claude's.
func (Adapter) Owns(command string) bool {
	p, ok := hooktypes.ParseCommand(command)
	return ok && (p.Agent == "" || p.Agent == Name)
}
