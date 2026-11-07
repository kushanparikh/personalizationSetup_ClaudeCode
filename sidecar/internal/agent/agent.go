// Package agent is the boundary between AI coding agents and the core. Each
// agent has an Adapter that turns its hook payload into one canonical Event;
// nothing after the adapter knows which agent sent it.
package agent

import "github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"

// Schema is the version of Event.
const Schema = 1

// Canonical event names.
const (
	SessionStart = "session_start"
	TaskDone     = "task_done"
	NeedsInput   = "needs_input"
	MovedOn      = "moved_on"
	SessionEnd   = "session_end"
)

// Event is what an adapter makes of one hook call.
type Event struct {
	Schema    int    `json:"schema"`
	Agent     string `json:"agent"`
	AgentName string `json:"agent_name"`
	Event     string `json:"event"`
	SessionID string `json:"session_id,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	Message   string `json:"message,omitempty"`
	// Cause is the agent's own name for the hook event behind a moved_on or
	// session_end (for example Claude's "Stop"). The app reads it unchanged.
	Cause string `json:"cause,omitempty"`
}

// Detection says whether an agent is installed and where its config lives.
type Detection struct {
	Installed  bool
	ConfigPath string
}

// Adapter is one agent's integration.
type Adapter interface {
	// DisplayName is the name shown in every alert title.
	DisplayName() string
	Detect() Detection
	// Capabilities lists the canonical events the agent can emit.
	Capabilities() []string
	// Install writes the entries of one hook type into the agent's config,
	// replacing that type's previous entries.
	Install(entries []hooktypes.Entry) error
	// Uninstall removes every entry this app added and reports how many,
	// plus the backup path ("" when nothing changed).
	Uninstall() (removed int, backup string, err error)
	// Parse reads the payload of hook subcommand sub. A bad payload still
	// yields a usable Event; hooks never fail.
	Parse(sub string, input []byte) Event
	// Owns reports whether a config entry's command was written by this app
	// for this agent.
	Owns(command string) bool
}

// Key identifies a session across agents, so two agents reusing one session
// id never share alerts. An empty id stays empty (a sessionless alert).
func Key(agent, sessionID string) string {
	if sessionID == "" {
		return ""
	}
	return agent + ":" + sessionID
}
