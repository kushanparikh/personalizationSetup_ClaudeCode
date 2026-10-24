package focus

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

type call struct {
	name string
	args []string
}

func fake(out string, err error, calls *[]call) Runner {
	return func(name string, args ...string) (string, error) {
		*calls = append(*calls, call{name, args})
		if name == "osascript" {
			return out, err
		}
		return "", nil
	}
}

func TestFocusTabFound(t *testing.T) {
	var calls []call
	got := Focus("/dev/ttys004", "com.apple.Terminal", fake("found", nil, &calls))
	if got != "tab" {
		t.Fatalf("got %q, want tab", got)
	}
	if len(calls) != 1 || calls[0].name != "osascript" {
		t.Fatalf("calls = %v", calls)
	}
	if !slices.Contains(calls[0].args, "/dev/ttys004") {
		t.Errorf("args lack tty: %v", calls[0].args)
	}
	if !slices.ContainsFunc(calls[0].args, func(a string) bool { return strings.Contains(a, "on run argv") }) {
		t.Errorf("args lack script: %v", calls[0].args)
	}
}

func TestFocusFallsBackToApp(t *testing.T) {
	var calls []call
	got := Focus("/dev/ttys004", "com.apple.Terminal", fake("notfound", nil, &calls))
	if got != "app" {
		t.Fatalf("got %q, want app", got)
	}
	last := calls[len(calls)-1]
	if last.name != "open" || !slices.Equal(last.args, []string{"-b", "com.apple.Terminal"}) {
		t.Errorf("last call = %v", last)
	}
}

func TestFocusOtherTerminal(t *testing.T) {
	var calls []call
	got := Focus("/dev/ttys004", "com.googlecode.iterm2", fake("found", nil, &calls))
	if got != "app" {
		t.Fatalf("got %q, want app", got)
	}
	if len(calls) != 1 || calls[0].name != "open" || !slices.Equal(calls[0].args, []string{"-b", "com.googlecode.iterm2"}) {
		t.Errorf("calls = %v", calls)
	}
}

func TestFocusScriptErrorFallsBackToApp(t *testing.T) {
	var calls []call
	if got := Focus("/dev/ttys004", "", fake("", errors.New("denied"), &calls)); got != "app" {
		t.Fatalf("got %q, want app", got)
	}
}

func TestFocusNone(t *testing.T) {
	run := func(string, ...string) (string, error) { return "", errors.New("fail") }
	if got := Focus("", "com.apple.Terminal", run); got != "none" {
		t.Fatalf("got %q, want none", got)
	}
}
