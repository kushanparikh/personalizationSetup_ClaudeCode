package hooktypes

import (
	"reflect"
	"strings"
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
		{Finished, Answers{Sound: "Glass", ClickToFocus: true}, []perms.ID{perms.AutomationTerminal}},
		{Finished, Answers{Sound: "Glass"}, []perms.ID{}},
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

func TestParseCommandEdgeCases(t *testing.T) {
	if _, ok := ParseCommand("~/.pcs/bin/pcs-hook hook "); ok {
		t.Error("empty sub parsed")
	}
	p, ok := ParseCommand("~/.pcs/bin/pcs-hook hook stop --sound")
	if !ok || p.Type != Finished || p.Answers.Sound != "" {
		t.Errorf("%+v ok=%v", p, ok)
	}
}

func TestNeedsFinishedNoNotifications(t *testing.T) {
	got := Needs(Finished, Answers{Sound: "Glass", ClickToFocus: true})
	if !reflect.DeepEqual(got, []perms.ID{perms.AutomationTerminal}) {
		t.Errorf("click-to-focus: got %#v", got)
	}
	if got := Needs(Finished, Answers{Sound: "Glass"}); len(got) != 0 {
		t.Errorf("no focus: got %#v", got)
	}
}

// A typo'd or mistyped question key would silently drop an answer in the form.
func TestQuestionKeysMatchAnswers(t *testing.T) {
	fields := map[string]reflect.Kind{}
	at := reflect.TypeOf(Answers{})
	for i := 0; i < at.NumField(); i++ {
		tag, _, _ := strings.Cut(at.Field(i).Tag.Get("json"), ",")
		fields[tag] = at.Field(i).Type.Kind()
	}
	want := map[string]reflect.Kind{KindBool: reflect.Bool, KindSound: reflect.String}
	for _, d := range All() {
		if len(d.Questions) == 0 {
			t.Errorf("%s has no questions", d.Type)
		}
		for _, q := range d.Questions {
			k, ok := fields[q.Key]
			if !ok {
				t.Errorf("%s: question key %q is not an Answers json tag", d.Type, q.Key)
				continue
			}
			wk, ok := want[q.Kind]
			if !ok {
				t.Errorf("%s: unknown question kind %q", d.Type, q.Kind)
			} else if wk != k {
				t.Errorf("%s: question %q kind %q needs a %v field, Answers has %v", d.Type, q.Key, q.Kind, wk, k)
			}
			if q.Label == "" {
				t.Errorf("%s: question %q has no label", d.Type, q.Key)
			}
		}
	}
}
