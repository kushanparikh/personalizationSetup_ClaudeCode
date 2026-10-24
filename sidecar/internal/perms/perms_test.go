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
