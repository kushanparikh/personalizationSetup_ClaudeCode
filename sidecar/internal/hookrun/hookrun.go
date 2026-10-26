// Package hookrun implements the `pcs hook <sub>` handlers. They never block
// Claude Code: every path returns exit code 0.
package hookrun

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/gitstatus"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/ipc"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/proc"
)

const (
	maxStdin      = 1 << 20
	maxMessage    = 1000
	defaultPrompt = "Claude needs your input"
)

// Input is the subset of Claude Code's hook JSON that the handlers use.
type Input struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Event     string `json:"hook_event_name"`
	Message   string `json:"message"`
}

// Deps are the side-effecting collaborators, injectable for tests.
type Deps struct {
	Send   func(ipc.Event, bool) error
	TTY    func() string
	Getenv func(string) string
	Git    func(cwd string, untrackedDirty bool) (string, error)
	Log    *slog.Logger
}

// DefaultDeps wires the real implementations.
func DefaultDeps() Deps {
	return Deps{
		Send:   ipc.DefaultClient().Send,
		TTY:    proc.SessionTTY,
		Getenv: os.Getenv,
		Git:    gitstatus.Summary,
		Log:    slog.Default(),
	}
}

// Run executes one hook subcommand and always returns 0.
func Run(sub string, args []string, stdin io.Reader, stdout io.Writer, d Deps) (code int) {
	var log *slog.Logger
	if d.Log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	} else {
		log = slog.New(safeHandler{d.Log.Handler()})
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("hook panic", "sub", sub, "panic", r)
			code = 0
		}
	}()

	var in Input
	if data, err := io.ReadAll(io.LimitReader(stdin, maxStdin)); err == nil {
		if err := json.Unmarshal(data, &in); err != nil {
			in = Input{}
		}
	}
	getenv := d.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	a := hooktypes.ParseArgs(sub, args)

	switch sub {
	case "session-start":
		line, err := d.Git(in.Cwd, a.UntrackedDirty)
		if err != nil {
			log.Warn("git summary failed", "err", err)
			return 0
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(map[string]string{"systemMessage": line}); err != nil {
			log.Warn("write failed", "err", err)
		}
	case "stop":
		e := ipc.Event{V: 1, Kind: ipc.KindToast, Title: title(in, getenv), Message: "Finished working", Tone: ipc.ToneDone,
			Sound: sound(a.Sound), Focus: a.ClickToFocus, SessionID: in.SessionID}
		if a.ClickToFocus {
			e.TTY = d.TTY()
			e.BundleID = proc.TerminalBundleID(getenv)
		}
		send(d, log, e, true)
	case "notification":
		msg := in.Message
		if msg == "" {
			msg = defaultPrompt
		}
		if r := []rune(msg); len(r) > maxMessage {
			msg = string(r[:maxMessage])
		}
		e := ipc.Event{V: 1, Kind: ipc.KindPanel, Title: title(in, getenv), Message: msg, Tone: ipc.ToneWait,
			Sound: sound(a.Sound), Focus: a.ClickToFocus, SessionID: in.SessionID}
		if a.ClickToFocus {
			e.TTY = d.TTY()
			e.BundleID = proc.TerminalBundleID(getenv)
		}
		if e.SessionID == "" {
			e.SessionID = e.TTY
		}
		if e.SessionID == "" {
			e.SessionID = "unknown"
		}
		send(d, log, e, true)
	case "moved-on":
		id := in.SessionID
		if id == "" {
			id = "unknown"
		}
		send(d, log, ipc.Event{V: 1, Kind: ipc.KindMovedOn, SessionID: id, Cause: cause(in.Event)}, false)
	default:
		log.Warn("unknown hook subcommand", "sub", sub)
	}
	return 0
}

// safeHandler swallows panics from the wrapped handler so logging can never
// break the always-exit-0 guarantee.
type safeHandler struct{ h slog.Handler }

func (s safeHandler) Enabled(ctx context.Context, l slog.Level) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return s.h.Enabled(ctx, l)
}

func (s safeHandler) Handle(ctx context.Context, r slog.Record) (err error) {
	defer func() { _ = recover() }()
	return s.h.Handle(ctx, r)
}

func (s safeHandler) WithAttrs(a []slog.Attr) (h slog.Handler) {
	defer func() {
		if recover() != nil {
			h = safeHandler{discardHandler{}}
		}
	}()
	return safeHandler{s.h.WithAttrs(a)}
}

func (s safeHandler) WithGroup(n string) (h slog.Handler) {
	defer func() {
		if recover() != nil {
			h = safeHandler{discardHandler{}}
		}
	}()
	return safeHandler{s.h.WithGroup(n)}
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }

func send(d Deps, log *slog.Logger, e ipc.Event, launch bool) {
	if err := d.Send(e, launch); err != nil {
		log.Warn("send failed", "kind", e.Kind, "err", err)
	}
}

// cause trims the hook event name to what ipc.Event.Validate accepts, so an
// odd name can never get the event dropped.
func cause(ev string) string {
	if r := []rune(ev); len(r) > 40 {
		return string(r[:40])
	}
	return ev
}

func sound(s string) string {
	if s == "none" {
		return ""
	}
	return s
}

func title(in Input, getenv func(string) string) string {
	p := ""
	if in.Cwd != "" {
		p = filepath.Base(in.Cwd)
	} else if pwd := getenv("PWD"); pwd != "" {
		p = filepath.Base(pwd)
	}
	if p == "" {
		p = "unknown"
	}
	return "Claude Code — " + p
}
