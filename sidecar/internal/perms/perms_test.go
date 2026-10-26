package perms

import "testing"

func TestStatusFromOSStatus(t *testing.T) {
	cases := map[int]Status{0: Granted, -1743: Denied, -1744: NotAsked, -600: Unknown, 42: Unknown}
	for code, want := range cases {
		if got := StatusFromOSStatus(code); got != want {
			t.Errorf("StatusFromOSStatus(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestStepsText(t *testing.T) {
	cases := map[ID]string{
		AutomationTerminal: "Open System Settings → Privacy & Security → Automation → Personalization Setup, and turn on Terminal.",
	}
	for id, want := range cases {
		if got := Lookup(id).Steps; got != want {
			t.Errorf("Lookup(%s).Steps = %q, want %q", id, got, want)
		}
	}
}

func TestRegistry(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("empty registry")
	}
	for _, p := range all {
		if p.Title == "" || p.Check == nil || p.Request == nil {
			t.Errorf("incomplete entry %+v", p)
		}
		if p.Askable && p.UnknownHint == "" {
			t.Errorf("%s: askable permission needs an UnknownHint", p.ID)
		}
	}
	a := Lookup(AutomationTerminal)
	if !a.Askable || a.UnknownHint != "Couldn't check this yet. Terminal may not be running. Click Ask for permission to check again." {
		t.Errorf("automation entry = %+v", a)
	}
}

func TestSetRegistryForTest(t *testing.T) {
	restore := SetRegistryForTest([]Info{{ID: "x-y", Title: "X", Check: func() Status { return Granted }}})
	if got := Lookup("x-y"); got.Title != "X" || got.Check() != Granted {
		t.Errorf("lookup = %+v", got)
	}
	restore()
	if Lookup("x-y").Title != "" {
		t.Error("registry not restored")
	}
}
