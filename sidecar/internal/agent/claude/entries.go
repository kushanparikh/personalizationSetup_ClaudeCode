package claude

import "github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"

const inputNeededMatcher = "permission_prompt|elicitation_dialog"

func soundFocusArgs(a hooktypes.Answers) string {
	focus := "--no-focus"
	if a.ClickToFocus {
		focus = "--focus"
	}
	return " --sound " + a.Sound + " " + focus
}

// Entries returns the settings.json hook entries for the type: which Claude
// Code events run which pcs hook commands. Commands are built only from
// validated enum values; invalid answers yield nil.
func Entries(t hooktypes.Type, a hooktypes.Answers) []hooktypes.Entry {
	if hooktypes.Validate(t, a) != nil {
		return nil
	}
	switch t {
	case hooktypes.GitStatus:
		cmd := hooktypes.CommandPrefix + "session-start"
		if !a.UntrackedDirty {
			cmd += " --untracked-clean"
		}
		return []hooktypes.Entry{{"SessionStart", "", cmd, 10}}
	case hooktypes.Finished:
		// revisit clears the session's finished light when the user goes
		// back to it. It is not moved-on, so the two types never own the
		// same command.
		revisit := hooktypes.CommandPrefix + "revisit"
		return []hooktypes.Entry{
			{"Stop", "", hooktypes.CommandPrefix + "stop" + soundFocusArgs(a), 10},
			{"UserPromptSubmit", "", revisit, 5},
			{"SessionEnd", "", revisit, 5},
		}
	case hooktypes.InputNeeded:
		moved := hooktypes.CommandPrefix + "moved-on"
		return []hooktypes.Entry{
			{"Notification", inputNeededMatcher, hooktypes.CommandPrefix + "notification" + soundFocusArgs(a), 10},
			{"UserPromptSubmit", "", moved, 5},
			{"PostToolUse", "", moved, 5},
			{"Stop", "", moved, 5},
		}
	}
	return nil
}
