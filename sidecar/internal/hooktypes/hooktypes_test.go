package hooktypes

import (
	"reflect"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/perms"
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
			[]Entry{{"Stop", "", p + "stop --sound Glass --focus", 10}}},
		{"finished none", Finished, Answers{Sound: "none", ClickToFocus: false},
			[]Entry{{"Stop", "", p + "stop --sound none --no-focus", 10}}},
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
	p, ok := ParseCommand("~/.pcs/bin/pcs-hook hook moved-on")
	if !ok || p.Type != InputNeeded || p.Sub != "moved-on" || p.Answers != (Answers{}) {
		t.Errorf("moved-on: %+v ok=%v", p, ok)
	}
	if _, ok := ParseCommand("~/.claude/hooks/notify-done.sh"); ok {
		t.Error("foreign command parsed")
	}
}

func TestValidateRejectsUnknownSound(t *testing.T) {
	if err := Validate(Finished, Answers{Sound: "Glass; rm -rf ~"}); err == nil {
		t.Error("want error")
	}
	if err := Validate(Finished, Answers{Sound: ""}); err == nil {
		t.Error("empty sound should be invalid")
	}
	if err := Validate("bogus", Answers{}); err == nil {
		t.Error("unknown type should error")
	}
	if err := Validate(Finished, Answers{Sound: "none"}); err != nil {
		t.Error(err)
	}
	if err := Validate(GitStatus, Answers{}); err != nil {
		t.Error(err)
	}
}

func TestNeeds(t *testing.T) {
	cases := []struct {
		typ  Type
		a    Answers
		want []perms.ID
	}{
		{Finished, Answers{Sound: "Glass", ClickToFocus: true}, []perms.ID{perms.Notifications, perms.AutomationTerminal}},
		{Finished, Answers{Sound: "Glass"}, []perms.ID{perms.Notifications}},
		{InputNeeded, Answers{Sound: "Ping", ClickToFocus: true}, []perms.ID{perms.AutomationTerminal}},
		{GitStatus, Answers{UntrackedDirty: true}, []perms.ID{}},
	}
	for _, c := range cases {
		if got := Needs(c.typ, c.a); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %#v want %#v", c.typ, got, c.want)
		}
	}
}

func TestAllAndGet(t *testing.T) {
	all := All()
	if len(all) != 3 || all[0].Type != GitStatus || all[1].Type != Finished || all[2].Type != InputNeeded {
		t.Fatalf("%+v", all)
	}
	d, ok := Get(Finished)
	if !ok || d.Defaults != (Answers{Sound: "Glass", ClickToFocus: true}) {
		t.Errorf("%+v", d)
	}
	if _, ok := Get("x"); ok {
		t.Error("unknown ok")
	}
}

func TestParseArgsDefaults(t *testing.T) {
	a := ParseArgs("stop", []string{"--sound", "Hero", "--bogus"})
	if a.Sound != "Hero" || !a.ClickToFocus {
		t.Errorf("%+v", a)
	}
	if a := ParseArgs("session-start", nil); !a.UntrackedDirty {
		t.Error("untracked default")
	}
}
