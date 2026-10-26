package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/focus"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/gitstatus"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/ipc"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/perms"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/report"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/settings"
)

const (
	testTitle   = "Claude Code — test"
	testMessage = "This is a test. Claude Code would show its question here."
)

func cmdPerm(args []string) (any, error) {
	if len(args) == 0 || args[0] != "request" {
		return nil, errors.New("usage: pcs perm request --id <permission>")
	}
	fs := newFlags("perm request")
	id := fs.String("id", "", "permission id")
	if err := parse(fs, args[1:]); err != nil {
		return nil, err
	}
	for _, p := range perms.All() {
		if p.ID == perms.ID(*id) && p.Askable {
			s := p.Request()
			slog.Default().Info("permission request", "id", *id, "status", s)
			return map[string]any{"status": s}, nil
		}
	}
	return nil, fmt.Errorf("unknown permission %q", *id)
}

func cmdFocus(args []string) (any, error) {
	fs := newFlags("focus")
	tty := fs.String("tty", "", "session tty")
	bundleID := fs.String("bundle-id", "", "terminal bundle id")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	t := *tty
	if !strings.HasPrefix(t, "/dev/") {
		t = ""
	}
	res := focus.Focus(t, *bundleID, focus.ExecRunner)
	slog.Default().Info("focus", "tty", t, "bundleId", *bundleID, "result", res)
	return map[string]any{"result": res}, nil
}

func cmdSendTest(args []string) (any, error) {
	fs := newFlags("send-test")
	typ := fs.String("type", "", "hook type")
	cwd := fs.String("cwd", "", "folder for the git-status preview")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	d, ok := hooktypes.Get(hooktypes.Type(*typ))
	if !ok {
		return nil, fmt.Errorf("unknown hook type %q", *typ)
	}
	f, err := settings.Load(paths.SettingsPath())
	if err != nil {
		f = nil // fall back to the stored answers
	}
	a, _ := report.CurrentAnswers(f, loadState(), d.Type)

	var ev ipc.Event
	switch d.Type {
	case hooktypes.GitStatus:
		dir := *cwd
		if dir == "" {
			dir = paths.Home()
		}
		line, err := gitstatus.Summary(dir, a.UntrackedDirty)
		if err != nil {
			return nil, err
		}
		return map[string]any{"preview": line}, nil
	case hooktypes.Finished:
		ev = ipc.Event{V: 1, Kind: ipc.KindToast, Title: testTitle, Message: "Finished working", Tone: ipc.ToneDone, Sound: soundName(a.Sound)}
	case hooktypes.InputNeeded:
		ev = ipc.Event{V: 1, Kind: ipc.KindPanel, Title: testTitle, Message: testMessage, Tone: ipc.ToneWait, Sound: soundName(a.Sound), SessionID: "test"}
	}
	if err := ipc.DefaultClient().Send(ev, false); err != nil {
		slog.Default().Warn("send-test delivery failed", "type", d.Type, "err", err)
		return nil, errors.New("the app isn't listening for hook events. Is it running?")
	}
	return map[string]any{"sent": true}, nil
}

func soundName(s string) string {
	if s == "none" {
		return ""
	}
	return s
}

func cmdServe(args []string) (any, error) {
	if err := parse(newFlags("serve"), args); err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	var once sync.Once
	closeStop := func() { once.Do(func() { close(stop) }) }
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin) // EOF: the parent app died
		closeStop()
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		closeStop()
	}()
	err := ipc.Serve(paths.SocketPath(), os.Stdout, stop)
	if errors.Is(err, ipc.ErrAlreadyRunning) {
		return nil, errors.New("already running")
	}
	return nil, err
}

func cmdInstallLauncher(args []string) (any, error) {
	fs := newFlags("install-launcher")
	bin := fs.String("bin", paths.AppBinary(), "binary the launcher runs (for development)")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	changed, err := installLauncher(*bin)
	if err != nil {
		return nil, err
	}
	return map[string]any{"changed": changed}, nil
}
