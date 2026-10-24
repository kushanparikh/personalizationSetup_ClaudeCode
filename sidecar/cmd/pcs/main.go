// Command pcs is the Personalization Setup sidecar. The app shell runs it
// for every action and reads one JSON object from stdout; Claude Code runs
// `pcs hook <sub>` (through the launcher) for every hook event.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hookrun"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/logx"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

// command runs one subcommand. A nil result prints nothing (used by serve,
// whose stdout is the event stream).
type command func(args []string) (any, error)

var commands = map[string]command{
	"state":            cmdState,
	"apply":            cmdApply,
	"enable":           cmdEnable,
	"disable":          cmdDisable,
	"remove":           cmdRemove,
	"perm":             cmdPerm,
	"focus":            cmdFocus,
	"send-test":        cmdSendTest,
	"serve":            cmdServe,
	"install-launcher": cmdInstallLauncher,
	"uninstall":        cmdUninstall,
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		runHook(os.Args[2:])
	}
	os.Exit(run(os.Args[1:]))
}

// runHook handles `pcs hook <sub>`. It always exits 0, even if setup panics.
func runHook(args []string) {
	defer func() {
		_ = recover()
		os.Exit(0)
	}()
	if paths.Home() == "" {
		return
	}
	slog.SetDefault(logx.Open())
	sub := ""
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	hookrun.Run(sub, args, os.Stdin, os.Stdout, hookrun.DefaultDeps())
}

func run(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Default().Error("command panicked", "args", args, "panic", r, "stack", string(debug.Stack()))
			code = fail(fmt.Errorf("internal error: %v", r))
		}
	}()
	if paths.Home() == "" {
		return fail(errors.New("can't find your home folder"))
	}
	slog.SetDefault(logx.Open())
	if len(args) == 0 {
		return fail(errors.New("usage: pcs <command> [flags]"))
	}
	cmd, ok := commands[args[0]]
	if !ok {
		return fail(fmt.Errorf("unknown command %q", args[0]))
	}
	res, err := cmd(args[1:])
	if err != nil {
		slog.Default().Error("command failed", "cmd", args[0], "err", err)
		return fail(err)
	}
	if res != nil {
		if err := writeJSON(res); err != nil {
			slog.Default().Error("write output failed", "cmd", args[0], "err", err)
			return 1
		}
	}
	return 0
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func fail(err error) int {
	_ = writeJSON(map[string]string{"error": err.Error()})
	return 1
}

// newFlags returns a flag set that reports errors instead of printing them.
func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parse parses args and rejects stray positional arguments.
func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s: %w", fs.Name(), err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%s: unexpected argument %q", fs.Name(), fs.Arg(0))
	}
	return nil
}
