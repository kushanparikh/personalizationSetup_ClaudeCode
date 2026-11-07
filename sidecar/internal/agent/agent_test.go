package agent

import "testing"

func TestKey(t *testing.T) {
	for _, c := range []struct{ agent, id, want string }{
		{"claude", "s1", "claude:s1"},
		{"kimi", "s1", "kimi:s1"},
		{"claude", "", ""},
	} {
		if got := Key(c.agent, c.id); got != c.want {
			t.Errorf("Key(%q, %q) = %q, want %q", c.agent, c.id, got, c.want)
		}
	}
}
