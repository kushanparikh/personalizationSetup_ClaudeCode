package hookrun

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/ipc"
)

type rec struct {
	events   []ipc.Event
	launches []bool
	ttyCalls int
	gitCalls int
	sendErr  error
	panicOn  bool
}

func (r *rec) deps(env map[string]string) Deps {
	return Deps{
		Send: func(e ipc.Event, l bool) error {
			if r.panicOn {
				panic("boom")
			}
			r.events = append(r.events, e)
			r.launches = append(r.launches, l)
			return r.sendErr
		},
		TTY:    func() string { r.ttyCalls++; return "/dev/ttys009" },
		Getenv: func(k string) string { return env[k] },
		Git: func(cwd string, u bool) (string, error) {
			r.gitCalls++
			return "📁 r  🌿 main  ✅ clean", nil
		},
	}
}

func run(t *testing.T, sub string, args []string, in string, d Deps) string {
	t.Helper()
	var out bytes.Buffer
	if code := Run(sub, args, strings.NewReader(in), &out, d); code != 0 {
		t.Fatalf("exit %d", code)
	}
	return out.String()
}

func TestSessionStartPrintsSystemMessage(t *testing.T) {
	r := &rec{}
	got := run(t, "session-start", nil, `{"cwd":"/x/r"}`, r.deps(nil))
	want := "{\"systemMessage\":\"📁 r  🌿 main  ✅ clean\"}\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSessionStartGitFailsPrintsNothing(t *testing.T) {
	r := &rec{}
	d := r.deps(nil)
	d.Git = func(string, bool) (string, error) { return "", errors.New("x") }
	if got := run(t, "session-start", nil, `{}`, d); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestStopToast(t *testing.T) {
	r := &rec{}
	run(t, "stop", []string{"--sound", "Glass", "--focus"}, `{"cwd":"/x/My Project"}`,
		r.deps(map[string]string{"__CFBundleIdentifier": "com.apple.Terminal", "TERM_PROGRAM": "Apple_Terminal"}))
	want := ipc.Event{V: 1, Kind: ipc.KindToast, Title: "Claude Code — My Project", Message: "Finished working", Tone: ipc.ToneDone,
		Sound: "Glass", Focus: true, TTY: "/dev/ttys009", BundleID: "com.apple.Terminal", SessionID: ""}
	if len(r.events) != 1 || !r.launches[0] {
		t.Fatalf("events %v launches %v", r.events, r.launches)
	}
	got := r.events[0]
	got.SessionID = ""
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestStopNoFocusNoTTY(t *testing.T) {
	r := &rec{}
	run(t, "stop", []string{"--sound", "none", "--no-focus"}, `{"cwd":"/x/p"}`, r.deps(nil))
	e := r.events[0]
	if e.Sound != "" || e.TTY != "" || e.BundleID != "" || e.Focus || r.ttyCalls != 0 {
		t.Fatalf("%+v tty=%d", e, r.ttyCalls)
	}
}

func TestNotificationPanel(t *testing.T) {
	r := &rec{}
	run(t, "notification", []string{"--sound", "Ping", "--focus"},
		`{"session_id":"s1","cwd":"/x/p","message":"hello"}`, r.deps(nil))
	e := r.events[0]
	if e.Kind != ipc.KindPanel || e.Message == "" || e.Message != "hello" || e.SessionID != "s1" || e.Tone != ipc.ToneWait || !r.launches[0] || e.Sound != "Ping" {
		t.Fatalf("%+v", e)
	}
}

func TestHookBadInput(t *testing.T) {
	for _, in := range []string{"", "not json", strings.Repeat("x", 2<<20)} {
		r := &rec{}
		run(t, "notification", nil, in, r.deps(map[string]string{"PWD": "/home/u/proj"}))
		e := r.events[0]
		if e.Message != "Claude needs your input" || e.Title != "Claude Code — proj" {
			t.Fatalf("%+v", e)
		}
		if e.SessionID != "/dev/ttys009" {
			t.Fatalf("session %q", e.SessionID)
		}
	}
	r := &rec{}
	run(t, "notification", []string{"--no-focus"}, "", r.deps(nil))
	if r.events[0].SessionID != "unknown" || r.events[0].Title != "Claude Code — unknown" {
		t.Fatalf("%+v", r.events[0])
	}
	r = &rec{}
	msg := strings.Repeat("é", 5000)
	run(t, "notification", nil, `{"message":"`+msg+`"}`, r.deps(nil))
	if n := len([]rune(r.events[0].Message)); n != 1000 {
		t.Fatalf("len %d", n)
	}
}

func TestMovedOnNeverLaunches(t *testing.T) {
	r := &rec{sendErr: errors.New("down")}
	run(t, "moved-on", nil, `{"session_id":"s1"}`, r.deps(nil))
	if r.launches[0] || r.ttyCalls != 0 || r.gitCalls != 0 {
		t.Fatalf("launch=%v tty=%d", r.launches, r.ttyCalls)
	}
	e := r.events[0]
	if e.Kind != ipc.KindMovedOn || e.SessionID != "s1" || e.Tone != "" || e.V != 1 {
		t.Fatalf("%+v", e)
	}
}

func TestPanicRecovered(t *testing.T) {
	r := &rec{panicOn: true}
	run(t, "stop", nil, `{}`, r.deps(nil))
}

func TestUnknownSub(t *testing.T) {
	r := &rec{}
	if got := run(t, "bogus", nil, `{}`, r.deps(nil)); got != "" || len(r.events) != 0 {
		t.Fatal("expected nothing")
	}
}

type panicHandler struct{}

func (panicHandler) Enabled(context.Context, slog.Level) bool  { panic("log") }
func (panicHandler) Handle(context.Context, slog.Record) error { panic("log") }
func (h panicHandler) WithAttrs([]slog.Attr) slog.Handler      { panic("log") }
func (h panicHandler) WithGroup(string) slog.Handler           { panic("log") }

func TestPanickingLoggerStillZero(t *testing.T) {
	for _, sub := range []string{"stop", "notification", "moved-on", "bogus"} {
		r := &rec{panicOn: true}
		d := r.deps(nil)
		d.Log = slog.New(panicHandler{})
		run(t, sub, nil, `{}`, d)
	}
}

func TestNoStdoutExceptSessionStart(t *testing.T) {
	for _, sub := range []string{"stop", "notification", "moved-on", "bogus"} {
		r := &rec{}
		if got := run(t, sub, nil, `{}`, r.deps(nil)); got != "" {
			t.Fatalf("%s wrote %q", sub, got)
		}
	}
}
