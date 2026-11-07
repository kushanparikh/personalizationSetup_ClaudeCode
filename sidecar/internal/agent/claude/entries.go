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
		return []hooktypes.Entry{{Event: "SessionStart", Matcher: "", Command: cmd, Timeout: 10}}
	case hooktypes.Finished:
		// revisit clears the session's finished light when the user goes
		// back to it. It is not moved-on, so the two types never own the
		// same command.
		revisit := hooktypes.CommandPrefix + "revisit"
		return []hooktypes.Entry{
			{Event: "Stop", Matcher: "", Command: hooktypes.CommandPrefix + "stop" + soundFocusArgs(a), Timeout: 10},
			{Event: "UserPromptSubmit", Matcher: "", Command: revisit, Timeout: 5},
			{Event: "SessionEnd", Matcher: "", Command: revisit, Timeout: 5},
		}
	case hooktypes.InputNeeded:
		moved := hooktypes.CommandPrefix + "moved-on"
		return []hooktypes.Entry{
			{Event: "Notification", Matcher: inputNeededMatcher, Command: hooktypes.CommandPrefix + "notification" + soundFocusArgs(a), Timeout: 10},
			{Event: "UserPromptSubmit", Matcher: "", Command: moved, Timeout: 5},
			{Event: "PostToolUse", Matcher: "", Command: moved, Timeout: 5},
			{Event: "Stop", Matcher: "", Command: moved, Timeout: 5},
		}
	}
	return nil
}
