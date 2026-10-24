// Package hooktypes is the registry of the v1 Claude Code hook types and the
// exact hook commands written into settings.json.
package hooktypes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/perms"
)

// Type identifies a hook type.
type Type string

const (
	GitStatus   Type = "git-status"
	Finished    Type = "finished"
	InputNeeded Type = "input-needed"
)

// CommandPrefix marks a settings.json hook command as owned by the app.
const CommandPrefix = paths.LauncherCommand + " hook "

const inputNeededMatcher = "permission_prompt|elicitation_dialog"

// Sounds are the allowed sound names; "none" is also valid and means silent.
var Sounds = []string{"Basso", "Blow", "Bottle", "Frog", "Funk", "Glass", "Hero", "Morse", "Ping", "Pop", "Purr", "Sosumi", "Submarine", "Tink"}

// Answers are the user's choices for a hook type. Unused fields are ignored.
type Answers struct {
	UntrackedDirty bool   `json:"untrackedDirty"`
	Sound          string `json:"sound"`
	ClickToFocus   bool   `json:"clickToFocus"`
}

// Entry is one hook entry to write into settings.json.
type Entry struct {
	Event, Matcher, Command string
	Timeout                 int
}

// Def describes a hook type.
type Def struct {
	Type        Type    `json:"type"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Defaults    Answers `json:"defaults"`
}

var defs = []Def{
	{GitStatus, "Git status on session start",
		"Shows the repo, branch and whether there are uncommitted changes when a Claude Code session starts.",
		Answers{UntrackedDirty: true}},
	{Finished, "Finished working",
		"A banner that slides away when Claude finishes. Click it to jump to that session's terminal tab.",
		Answers{Sound: "Glass", ClickToFocus: true}},
	{InputNeeded, "Input needed",
		"A panel that stays on screen while Claude is waiting on you. Click it to jump to that session's terminal tab.",
		Answers{Sound: "Ping", ClickToFocus: true}},
}

// All returns every hook type in display order.
func All() []Def { return slices.Clone(defs) }

// Get returns the Def for t.
func Get(t Type) (Def, bool) {
	for _, d := range defs {
		if d.Type == t {
			return d, true
		}
	}
	return Def{}, false
}

func validSound(s string) bool { return s == "none" || slices.Contains(Sounds, s) }

// Validate rejects an unknown type or a sound outside the allowed list.
func Validate(t Type, a Answers) error {
	if _, ok := Get(t); !ok {
		return fmt.Errorf("unknown hook type %q", t)
	}
	if t != GitStatus && !validSound(a.Sound) {
		return fmt.Errorf("invalid sound %q", a.Sound)
	}
	return nil
}

// Needs lists the permissions the type needs with these answers.
func Needs(t Type, a Answers) []perms.ID {
	need := []perms.ID{}
	switch t {
	case Finished:
		need = append(need, perms.Notifications)
		if a.ClickToFocus {
			need = append(need, perms.AutomationTerminal)
		}
	case InputNeeded:
		if a.ClickToFocus {
			need = append(need, perms.AutomationTerminal)
		}
	}
	return need
}

func soundFocusArgs(a Answers) string {
	focus := "--no-focus"
	if a.ClickToFocus {
		focus = "--focus"
	}
	return " --sound " + a.Sound + " " + focus
}

// Entries returns the settings.json hook entries for the type. Commands are
// built only from validated enum values.
func Entries(t Type, a Answers) []Entry {
	switch t {
	case GitStatus:
		cmd := CommandPrefix + "session-start"
		if !a.UntrackedDirty {
			cmd += " --untracked-clean"
		}
		return []Entry{{"SessionStart", "", cmd, 10}}
	case Finished:
		return []Entry{{"Stop", "", CommandPrefix + "stop" + soundFocusArgs(a), 10}}
	case InputNeeded:
		moved := CommandPrefix + "moved-on"
		return []Entry{
			{"Notification", inputNeededMatcher, CommandPrefix + "notification" + soundFocusArgs(a), 10},
			{"UserPromptSubmit", "", moved, 5},
			{"PostToolUse", "", moved, 5},
			{"Stop", "", moved, 5},
		}
	}
	return nil
}

// Parsed is the result of recognising an app-owned hook command.
type Parsed struct {
	Type    Type
	Sub     string
	Answers Answers
}

// ParseCommand recognises an app-owned hook command.
func ParseCommand(cmd string) (Parsed, bool) {
	rest, ok := strings.CutPrefix(cmd, CommandPrefix)
	if !ok {
		return Parsed{}, false
	}
	fields := strings.Split(rest, " ")
	sub := fields[0]
	var t Type
	switch sub {
	case "session-start":
		t = GitStatus
	case "stop":
		t = Finished
	case "notification", "moved-on":
		t = InputNeeded
	default:
		return Parsed{}, false
	}
	return Parsed{Type: t, Sub: sub, Answers: ParseArgs(sub, fields[1:])}, true
}

// ParseArgs reads a hook subcommand's flags. Unknown flags are ignored and
// fields the subcommand does not use stay zero.
func ParseArgs(sub string, args []string) Answers {
	var a Answers
	switch sub {
	case "session-start":
		a.UntrackedDirty = !slices.Contains(args, "--untracked-clean")
	case "stop", "notification":
		a.ClickToFocus = true
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--sound":
				if i+1 < len(args) {
					a.Sound = args[i+1]
					i++
				}
			case "--focus":
				a.ClickToFocus = true
			case "--no-focus":
				a.ClickToFocus = false
			}
		}
	}
	return a
}
