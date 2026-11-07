package claude

import (
	"reflect"
	"testing"

	// The tests moved here from hooktypes with Entries; the dot import keeps
	// their tables readable.
	. "github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
)

func TestEntriesExact(t *testing.T) {
	const p = "~/.pcs/bin/pcs-hook hook "
	cases := []struct {
		name string
		typ  Type
		a    Answers
		want []Entry
	}{
		{"git default", GitStatus, Answers{UntrackedDirty: true},
			[]Entry{{"SessionStart", "", p + "session-start", 10}}},
		{"git clean", GitStatus, Answers{UntrackedDirty: false},
			[]Entry{{"SessionStart", "", p + "session-start --untracked-clean", 10}}},
		{"finished default", Finished, Answers{Sound: "Glass", ClickToFocus: true},
			[]Entry{
				{"Stop", "", p + "stop --sound Glass --focus", 10},
				{"UserPromptSubmit", "", p + "revisit", 5},
				{"SessionEnd", "", p + "revisit", 5},
			}},
		{"finished none", Finished, Answers{Sound: "none", ClickToFocus: false},
			[]Entry{
				{"Stop", "", p + "stop --sound none --no-focus", 10},
				{"UserPromptSubmit", "", p + "revisit", 5},
				{"SessionEnd", "", p + "revisit", 5},
			}},
		{"input default", InputNeeded, Answers{Sound: "Ping", ClickToFocus: true}, []Entry{
			{"Notification", "permission_prompt|elicitation_dialog", p + "notification --sound Ping --focus", 10},
			{"UserPromptSubmit", "", p + "moved-on", 5},
			{"PostToolUse", "", p + "moved-on", 5},
			{"Stop", "", p + "moved-on", 5},
		}},
	}
	for _, c := range cases {
		if got := Entries(c.typ, c.a); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %#v want %#v", c.name, got, c.want)
		}
	}
}

func TestParseCommandRoundTrip(t *testing.T) {
	sets := map[Type][]Answers{
		GitStatus:   {{UntrackedDirty: true}, {UntrackedDirty: false}},
		Finished:    {{Sound: "Glass", ClickToFocus: true}, {Sound: "none"}, {Sound: "Tink", ClickToFocus: true}},
		InputNeeded: {{Sound: "Ping", ClickToFocus: true}, {Sound: "Hero"}},
	}
	for typ, list := range sets {
		for _, a := range list {
			p, ok := ParseCommand(Entries(typ, a)[0].Command)
			if !ok || p.Type != typ || p.Answers != a {
				t.Errorf("%s %+v: got %+v ok=%v", typ, a, p, ok)
			}
		}
	}
}

func TestEntriesRejectsInvalid(t *testing.T) {
	bad := []struct {
		typ Type
		a   Answers
	}{
		{Finished, Answers{Sound: "Glass; rm -rf ~"}},
		{Finished, Answers{}},
		{InputNeeded, Answers{}},
		{InputNeeded, Answers{Sound: "$(x)"}},
		{"bogus", Answers{Sound: "Glass"}},
	}
	for _, c := range bad {
		if got := Entries(c.typ, c.a); got != nil {
			t.Errorf("%s %+v: got %#v want nil", c.typ, c.a, got)
		}
	}
}

func TestFinishedAndInputNeededCommandsDiffer(t *testing.T) {
	a := Answers{Sound: "Glass", ClickToFocus: true}
	for _, f := range Entries(Finished, a) {
		for _, in := range Entries(InputNeeded, a) {
			if f.Command == in.Command {
				t.Errorf("finished and input-needed share command %q", f.Command)
			}
		}
	}
}
