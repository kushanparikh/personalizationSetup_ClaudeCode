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
