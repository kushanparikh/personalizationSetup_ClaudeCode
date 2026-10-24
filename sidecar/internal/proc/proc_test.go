package proc

import "testing"

func TestFindTTYWalksUp(t *testing.T) {
	rows := []Row{{500, 400, "??"}, {400, 300, "??"}, {300, 1, "ttys003"}, {1, 0, "??"}}
	if got := FindTTY(rows, 500); got != "/dev/ttys003" {
		t.Fatalf("got %q", got)
	}
	if got := FindTTY(rows, 300); got != "/dev/ttys003" {
		t.Fatalf("start row own tty: got %q", got)
	}
	if got := FindTTY([]Row{{3, 1, "/dev/ttys009"}}, 3); got != "/dev/ttys009" {
		t.Fatalf("double prefix: got %q", got)
	}
}

func TestFindTTYNone(t *testing.T) {
	rows := []Row{{500, 400, "??"}, {400, 1, "??"}}
	if got := FindTTY(rows, 500); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := FindTTY(rows, 999); got != "" {
		t.Fatalf("missing pid: got %q", got)
	}
}

func TestFindTTYLoop(t *testing.T) {
	rows := []Row{{5, 6, "??"}, {6, 5, "??"}}
	if got := FindTTY(rows, 5); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestParsePSPadding(t *testing.T) {
	out := []byte("    1     0 ??\n  300     1 ttys003\n12345   300 ttys003\nbad line\nx 1 ??\n  7 y ??\n\n")
	rows := ParsePS(out)
	want := []Row{{1, 0, "??"}, {300, 1, "ttys003"}, {12345, 300, "ttys003"}}
	if len(rows) != len(want) {
		t.Fatalf("got %v", rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row %d: got %v want %v", i, rows[i], want[i])
		}
	}
}

func TestTerminalBundleID(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	if got := TerminalBundleID(env(map[string]string{"__CFBundleIdentifier": "x.y", "TERM_PROGRAM": "vscode"})); got != "x.y" {
		t.Fatalf("env first: %q", got)
	}
	cases := map[string]string{
		"Apple_Terminal": "com.apple.Terminal",
		"iTerm.app":      "com.googlecode.iterm2",
		"vscode":         "com.microsoft.VSCode",
		"ghostty":        "com.mitchellh.ghostty",
		"other":          "",
		"":               "",
	}
	for tp, want := range cases {
		if got := TerminalBundleID(env(map[string]string{"TERM_PROGRAM": tp})); got != want {
			t.Fatalf("%q: got %q want %q", tp, got, want)
		}
	}
}

func TestSessionTTYDoesNotPanic(t *testing.T) {
	_ = SessionTTY()
}
