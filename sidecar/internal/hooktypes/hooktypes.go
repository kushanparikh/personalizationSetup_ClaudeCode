// Package hooktypes is the registry of the v1 hook types, their answers, and
// the `pcs hook` command grammar. Which agent events run those commands is
// each agent adapter's business (see internal/agent).
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

// Sounds are the allowed sound names; "none" is also valid and means silent.
var Sounds = []string{"Basso", "Blow", "Bottle", "Frog", "Funk", "Glass", "Hero", "Morse", "Ping", "Pop", "Purr", "Sosumi", "Submarine", "Tink"}

// Answers are the user's choices for a hook type. Unused fields are ignored.
type Answers struct {
	UntrackedDirty bool   `json:"untrackedDirty"`
	Sound          string `json:"sound"`
	ClickToFocus   bool   `json:"clickToFocus"`
}

// Entry is one hook entry to write into an agent's config.
type Entry struct {
	Event, Matcher, Command string
	Timeout                 int
}

// Question kinds the settings form knows how to render.
const (
	KindBool  = "bool"
	KindSound = "sound"
)

// Question is one thing the settings form asks. Key is the json tag of an
// Answers field; Kind says which control edits it.
type Question struct {
	Key   string `json:"key"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// Def describes a hook type.
type Def struct {
	Type        Type       `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Defaults    Answers    `json:"defaults"`
	Questions   []Question `json:"questions"`
	Note        string     `json:"note,omitempty"`
}

var notifyQuestions = []Question{
	{"sound", KindSound, "Sound"},
	{"clickToFocus", KindBool, "Click focuses the session's terminal tab"},
}

const spacesNote = "If that session's window is on another desktop or full-screen Space, macOS won't switch to it. The tab is selected and Terminal comes forward."

var defs = []Def{
	{GitStatus, "Git status on session start",
		"Shows the repo, branch and whether there are uncommitted changes when a Claude Code session starts.",
		Answers{UntrackedDirty: true},
		[]Question{{"untrackedDirty", KindBool, "Count untracked files as dirty"}}, ""},
	{Finished, "Finished working",
		"A banner that slides away when Claude finishes. Click it to jump to that session's terminal tab.",
		Answers{Sound: "Glass", ClickToFocus: true}, notifyQuestions, spacesNote},
	{InputNeeded, "Input needed",
		"A panel that stays on screen while Claude is waiting on you. Click it to jump to that session's terminal tab.",
		Answers{Sound: "Ping", ClickToFocus: true}, notifyQuestions, spacesNote},
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
	case Finished, InputNeeded:
		if a.ClickToFocus {
			need = append(need, perms.AutomationTerminal)
		}
	}
	return need
}

// Parsed is the result of recognising an app-owned hook command.
type Parsed struct {
	// Agent is the --agent value before the subcommand; "" when absent.
	Agent   string
	Type    Type
	Sub     string
	Answers Answers
}

// CarriesAnswers reports whether the command holds its type's answers.
// moved-on and revisit entries carry none.
func (p Parsed) CarriesAnswers() bool { return p.Sub != "moved-on" && p.Sub != "revisit" }

// ParseCommand recognises an app-owned hook command, with an optional
// `--agent <name>` before the subcommand.
func ParseCommand(cmd string) (Parsed, bool) {
	rest, ok := strings.CutPrefix(cmd, CommandPrefix)
	if !ok {
		return Parsed{}, false
	}
	fields := strings.Fields(rest)
	agent := ""
	if len(fields) > 0 {
		if v, ok := strings.CutPrefix(fields[0], "--agent="); ok {
			agent, fields = v, fields[1:]
		} else if fields[0] == "--agent" && len(fields) > 1 {
			agent, fields = fields[1], fields[2:]
		}
	}
	if len(fields) == 0 {
		return Parsed{}, false
	}
	sub := fields[0]
	var t Type
	switch sub {
	case "session-start":
		t = GitStatus
	case "stop", "revisit":
		t = Finished
	case "notification", "moved-on":
		t = InputNeeded
	default:
		return Parsed{}, false
	}
	return Parsed{Agent: agent, Type: t, Sub: sub, Answers: ParseArgs(sub, fields[1:])}, true
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
