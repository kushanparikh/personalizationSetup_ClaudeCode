// Package hookrun implements the `pcs hook <sub>` handlers. An agent adapter
// turns the payload into a canonical event; the handlers never block the
// agent: every path returns exit code 0.
package hookrun

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/agent"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/gitstatus"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/ipc"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/proc"
)

const (
	maxStdin   = 1 << 20
	maxMessage = 1000
)

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

// Run executes one hook subcommand for the agent behind ad and always
// returns 0.
func Run(ad agent.Adapter, sub string, args []string, stdin io.Reader, stdout io.Writer, d Deps) (code int) {
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

	data, err := io.ReadAll(io.LimitReader(stdin, maxStdin))
	if err != nil {
		data = nil
	}
	in := ad.Parse(sub, data)
	getenv := d.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	a := hooktypes.ParseArgs(sub, args)

	switch in.Event {
	case agent.SessionStart:
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
	case agent.TaskDone:
		e := ipc.Event{V: 1, Kind: ipc.KindToast, Title: title(in, getenv), Message: "Finished working", Tone: ipc.ToneDone,
			Sound: sound(a.Sound), Focus: a.ClickToFocus, SessionID: agent.Key(in.Agent, in.SessionID)}
		if a.ClickToFocus {
			e.TTY = d.TTY()
			e.BundleID = proc.TerminalBundleID(getenv)
		}
		send(d, log, e, true)
	case agent.NeedsInput:
		msg := in.Message
		if msg == "" {
			msg = in.AgentName + " needs your input"
		}
		if r := []rune(msg); len(r) > maxMessage {
			msg = string(r[:maxMessage])
		}
		e := ipc.Event{V: 1, Kind: ipc.KindPanel, Title: title(in, getenv), Message: msg, Tone: ipc.ToneWait,
			Sound: sound(a.Sound), Focus: a.ClickToFocus}
		if a.ClickToFocus {
			e.TTY = d.TTY()
			e.BundleID = proc.TerminalBundleID(getenv)
		}
		id := in.SessionID
		if id == "" {
			id = e.TTY
		}
		if id == "" {
			id = "unknown"
		}
		e.SessionID = agent.Key(in.Agent, id)
		send(d, log, e, true)
	case agent.MovedOn, agent.SessionEnd:
		id := in.SessionID
		if id == "" {
			id = "unknown"
		}
		send(d, log, ipc.Event{V: 1, Kind: ipc.KindMovedOn, SessionID: agent.Key(in.Agent, id), Cause: cause(in.Cause)}, false)
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

// title is "<agent display name> — <folder>".
func title(in agent.Event, getenv func(string) string) string {
	p := ""
	if in.Cwd != "" {
		p = filepath.Base(in.Cwd)
	} else if pwd := getenv("PWD"); pwd != "" {
		p = filepath.Base(pwd)
	}
	if p == "" {
		p = "unknown"
	}
	return in.AgentName + " — " + p
}
