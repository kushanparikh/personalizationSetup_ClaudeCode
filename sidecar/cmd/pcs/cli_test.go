package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var pcsBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pcs-cli-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pcsBin = filepath.Join(dir, "pcs")
	if out, err := exec.Command("go", "build", "-o", pcsBin, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build pcs: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// userSettings is the user's real settings shape, with the 3 hand-built hooks.
const userSettings = `{
  "attribution": {
    "commit": "",
    "pr": ""
  },
  "model": "opus",
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/git-status.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ]
  },
  "enabledPlugins": {
    "superpowers@claude-plugins-official": true,
    "andrej-karpathy-skills@karpathy-skills": false
  },
  "extraKnownMarketplaces": {
    "karpathy-skills": {
      "source": {
        "source": "github",
        "repo": "forrestchang/andrej-karpathy-skills"
      }
    }
  },
  "tui": "fullscreen",
  "theme": "auto"
}
`

type env struct {
	t    *testing.T
	home string
	path string // PATH override; "" keeps the inherited PATH
}

func newEnv(t *testing.T, settingsContent string) *env {
	t.Helper()
	e := &env{t: t, home: t.TempDir()}
	if settingsContent != "" {
		if err := os.MkdirAll(filepath.Dir(e.settingsPath()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(e.settingsPath(), []byte(settingsContent), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *env) settingsPath() string { return filepath.Join(e.home, ".claude", "settings.json") }

func (e *env) settings() string {
	e.t.Helper()
	b, err := os.ReadFile(e.settingsPath())
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

func (e *env) environ() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOME=") || (e.path != "" && strings.HasPrefix(kv, "PATH=")) {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "HOME="+e.home)
	if e.path != "" {
		out = append(out, "PATH="+e.path)
	}
	return out
}

// run executes pcs and returns stdout and the exit code.
func (e *env) run(stdin string, args ...string) (string, int) {
	e.t.Helper()
	cmd := exec.Command(pcsBin, args...)
	cmd.Env = e.environ()
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		e.t.Fatalf("run pcs %v: %v", args, err)
	}
	if esc := "\\" + "u00"; strings.Contains(out.String(), esc+"26") || strings.Contains(out.String(), esc+"3c") || strings.Contains(out.String(), esc+"3e") {
		e.t.Fatalf("pcs %v: HTML-escaped output: %s", args, out.String())
	}
	return out.String(), code
}

// ok runs pcs, requires exit 0 and decodes the JSON object it printed.
func (e *env) ok(args ...string) map[string]any {
	e.t.Helper()
	out, code := e.run("", args...)
	if code != 0 {
		e.t.Fatalf("pcs %v: exit %d: %s", args, code, out)
	}
	return decode(e.t, out)
}

// fail runs pcs, requires exit 1 and returns the error message.
func (e *env) fail(args ...string) string {
	e.t.Helper()
	out, code := e.run("", args...)
	if code != 1 {
		e.t.Fatalf("pcs %v: exit %d, want 1: %s", args, code, out)
	}
	m := decode(e.t, out)
	msg, _ := m["error"].(string)
	if msg == "" || len(m) != 1 {
		e.t.Fatalf("pcs %v: want {\"error\":...}, got %s", args, out)
	}
	return msg
}

func decode(t *testing.T, out string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not a JSON object: %q: %v", out, err)
	}
	return m
}

func hookByType(t *testing.T, rep map[string]any, typ string) map[string]any {
	t.Helper()
	hooks, _ := rep["hooks"].([]any)
	for _, h := range hooks {
		if m, _ := h.(map[string]any); m["type"] == typ {
			return m
		}
	}
	return nil
}

func (e *env) stateHooks() map[string]any {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.home, ".pcs", "state.json"))
	if err != nil {
		e.t.Fatal(err)
	}
	var s struct {
		Hooks map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		e.t.Fatal(err)
	}
	return s.Hooks
}

func TestApplyDisableEnableRemove(t *testing.T) {
	for _, tc := range []struct{ typ, answers, cmd string }{
		{"finished", `{"sound":"Hero","clickToFocus":true}`, "~/.pcs/bin/pcs-hook hook stop --sound Hero --focus"},
		{"input-needed", `{"sound":"Pop","clickToFocus":false}`, "~/.pcs/bin/pcs-hook hook notification --sound Pop --no-focus"},
		{"git-status", `{"untrackedDirty":false}`, "~/.pcs/bin/pcs-hook hook session-start --untracked-clean"},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			e := newEnv(t, userSettings)

			rep := e.ok("apply", "--type", tc.typ, "--answers", tc.answers)
			h := hookByType(t, rep, tc.typ)
			if h == nil || h["enabled"] != true {
				t.Fatalf("after apply: %v", rep)
			}
			if !strings.Contains(e.settings(), `"command": "`+tc.cmd+`"`) {
				t.Fatalf("settings missing %q:\n%s", tc.cmd, e.settings())
			}
			if fi, err := os.Stat(filepath.Join(e.home, ".pcs", "bin", "pcs-hook")); err != nil || fi.Mode().Perm() != 0o755 {
				t.Fatalf("launcher not installed: %v", err)
			}
			if _, ok := e.stateHooks()[tc.typ]; !ok {
				t.Fatal("answers not stored")
			}

			rep = e.ok("disable", "--type", tc.typ)
			if h := hookByType(t, rep, tc.typ); h == nil || h["enabled"] != false || h["status"] != "off" {
				t.Fatalf("after disable: %v", rep)
			}
			if e.settings() != userSettings {
				t.Fatalf("disable left owned entries:\n%s", e.settings())
			}

			rep = e.ok("enable", "--type", tc.typ)
			if h := hookByType(t, rep, tc.typ); h == nil || h["enabled"] != true {
				t.Fatalf("after enable: %v", rep)
			}
			if !strings.Contains(e.settings(), tc.cmd) {
				t.Fatalf("enable did not restore the stored answers:\n%s", e.settings())
			}

			rep = e.ok("remove", "--type", tc.typ)
			if h := hookByType(t, rep, tc.typ); h != nil {
				t.Fatalf("after remove: %v", rep)
			}
			if got := e.settings(); got != userSettings {
				t.Fatalf("after remove settings differ:\n%s", got)
			}
			if _, ok := e.stateHooks()[tc.typ]; ok {
				t.Fatal("state still has the type")
			}
		})
	}
}

func TestApplyRejectsBadAnswers(t *testing.T) {
	e := newEnv(t, userSettings)
	msg := e.fail("apply", "--type", "finished", "--answers", `{"sound":"x"}`)
	if !strings.Contains(msg, "sound") {
		t.Fatalf("error = %q", msg)
	}
	e.fail("apply", "--type", "nope", "--answers", `{}`)
	e.fail("apply", "--type", "finished", "--answers", `not json`)
	if e.settings() != userSettings {
		t.Fatalf("settings changed:\n%s", e.settings())
	}
}

func TestApplyRefusesInvalidSettings(t *testing.T) {
	const bad = "{\n  \"model\": \"opus\",\n"
	e := newEnv(t, bad)
	msg := e.fail("apply", "--type", "finished", "--answers", `{"sound":"Glass","clickToFocus":true}`)
	if !strings.Contains(msg, "isn't valid JSON") {
		t.Fatalf("error = %q", msg)
	}
	if e.settings() != bad {
		t.Fatal("invalid settings were changed")
	}
	rep := e.ok("state")
	if se, _ := rep["settingsError"].(string); !strings.Contains(se, "isn't valid JSON") {
		t.Fatalf("state report = %v", rep)
	}
	e.fail("uninstall", "--yes")
	if e.settings() != bad {
		t.Fatal("uninstall changed invalid settings")
	}
}

func TestApplyRefusesUneditableHooks(t *testing.T) {
	const odd = "{\n  \"hooks\": []\n}\n"
	e := newEnv(t, odd)
	msg := e.fail("apply", "--type", "finished", "--answers", `{"sound":"Glass","clickToFocus":true}`)
	want := `~/.claude/settings.json has a "hooks" section this app can't safely edit, so Finished working wasn't added. Nothing was changed.`
	if msg != want {
		t.Fatalf("error = %q", msg)
	}
	if e.settings() != odd {
		t.Fatal("settings changed")
	}

	// An owned entry already exists, but one target event is malformed:
	// SetOwned changes nothing, and the stale entry must not pass as success.
	const partial = `{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt|elicitation_dialog",
        "hooks": [
          {
            "type": "command",
            "command": "~/.pcs/bin/pcs-hook hook notification --sound Ping --focus",
            "timeout": 10
          }
        ]
      }
    ],
    "PostToolUse": {}
  }
}
`
	e = newEnv(t, partial)
	msg = e.fail("apply", "--type", "input-needed", "--answers", `{"sound":"Pop","clickToFocus":true}`)
	if !strings.Contains(msg, "so Input needed wasn't added") {
		t.Fatalf("error = %q", msg)
	}
	if e.settings() != partial {
		t.Fatal("settings changed")
	}
}

func TestStateReport(t *testing.T) {
	e := newEnv(t, userSettings)
	rep := e.ok("state", "--notifications", "granted")
	if other, _ := rep["otherHooks"].([]any); len(other) != 3 {
		t.Fatalf("otherHooks = %v", rep["otherHooks"])
	}
	if av, _ := rep["available"].([]any); len(av) != 3 {
		t.Fatalf("available = %v", rep["available"])
	}
	if lp, _ := rep["logPath"].(string); lp != filepath.Join(e.home, "Library", "Logs", "Personalization Setup", "pcs.log") {
		t.Fatalf("logPath = %q", lp)
	}
	e.fail("state", "--notifications", "maybe")
}

func TestUninstall(t *testing.T) {
	e := newEnv(t, userSettings)
	e.ok("apply", "--type", "finished", "--answers", `{"sound":"Glass","clickToFocus":true}`)
	e.ok("apply", "--type", "input-needed", "--answers", `{"sound":"Ping","clickToFocus":true}`)
	e.fail("uninstall")

	res := e.ok("uninstall", "--yes")
	if n, _ := res["removedHooks"].(float64); n != 5 {
		t.Fatalf("removedHooks = %v", res)
	}
	backup, _ := res["backup"].(string)
	if filepath.Dir(backup) != filepath.Join(e.home, ".claude") {
		t.Fatalf("backup = %q", backup)
	}
	if b, err := os.ReadFile(backup); err != nil || !strings.Contains(string(b), "pcs-hook") {
		t.Fatalf("backup unreadable or not the pre-uninstall file: %v", err)
	}
	if e.settings() != userSettings {
		t.Fatalf("uninstall touched more than owned entries:\n%s", e.settings())
	}
	for _, d := range []string{filepath.Join(e.home, ".pcs"), filepath.Join(e.home, "Library", "Logs", "Personalization Setup")} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", d, err)
		}
	}
}

func TestHookAlwaysZero(t *testing.T) {
	e := newEnv(t, "")
	e.path = t.TempDir() // no `open`, no `ps`
	start := time.Now()
	out, code := e.run(`{"session_id":"s1","cwd":"/tmp/x","message":"hi"}`, "hook", "notification", "--sound", "Ping", "--focus")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "" {
		t.Fatalf("stdout = %q", out)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("took %v", d)
	}
	if _, code := e.run("garbage", "hook"); code != 0 {
		t.Fatalf("bare hook exit %d", code)
	}
}

func TestErrors(t *testing.T) {
	e := newEnv(t, userSettings)
	e.fail()
	e.fail("bogus")
	if msg := e.fail("perm", "request", "--id", "notifications"); msg != "notifications are requested by the app" {
		t.Fatalf("error = %q", msg)
	}
	if msg := e.fail("send-test", "--type", "finished"); msg != "the app isn't listening for hook events. Is it running?" {
		t.Fatalf("error = %q", msg)
	}
}

func TestSendTestGitPreview(t *testing.T) {
	e := newEnv(t, userSettings)
	res := e.ok("send-test", "--type", "git-status", "--cwd", t.TempDir())
	if res["preview"] != "📁 Not a git repository" {
		t.Fatalf("preview = %v", res)
	}
}

func TestInstallLauncher(t *testing.T) {
	e := newEnv(t, "")
	if res := e.ok("install-launcher"); res["changed"] != true {
		t.Fatalf("first: %v", res)
	}
	if res := e.ok("install-launcher"); res["changed"] != false {
		t.Fatalf("second: %v", res)
	}
	b, err := os.ReadFile(filepath.Join(e.home, ".pcs", "bin", "pcs-hook"))
	want := `BIN="` + filepath.Join(e.home, "Applications", "Personalization Setup.app", "Contents", "MacOS", "pcs") + `"`
	if err != nil || !strings.Contains(string(b), want) {
		t.Fatalf("launcher: %v\n%s", err, b)
	}
}

func TestWriteCommandsKeepExistingLauncher(t *testing.T) {
	e := newEnv(t, userSettings)
	e.ok("install-launcher", "--bin", "/x/pcs")
	e.ok("apply", "--type", "finished", "--answers", `{"sound":"Glass","clickToFocus":true}`)
	e.ok("disable", "--type", "finished")
	b, err := os.ReadFile(filepath.Join(e.home, ".pcs", "bin", "pcs-hook"))
	if err != nil || !strings.Contains(string(b), `BIN="/x/pcs"`) {
		t.Fatalf("dev launcher overwritten: %v\n%s", err, b)
	}
	if res := e.ok("install-launcher"); res["changed"] != true {
		t.Fatalf("install-launcher must always rewrite: %v", res)
	}
}

func TestPanicBecomesError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	commands["test-panic"] = func([]string) (any, error) { panic("boom") }
	defer delete(commands, "test-panic")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	code := run([]string{"test-panic"})
	os.Stdout = stdout
	w.Close()
	var out bytes.Buffer
	out.ReadFrom(r)
	if code != 1 || strings.TrimSpace(out.String()) != `{"error":"internal error: boom"}` {
		t.Fatalf("code %d, out %q", code, out.String())
	}
}

func TestUninstallRemovesUnknownOwnedEntries(t *testing.T) {
	const in = `{
  "model": "opus",
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.pcs/bin/pcs-hook hook future-thing",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
`
	e := newEnv(t, in)
	res := e.ok("uninstall", "--yes")
	if n, _ := res["removedHooks"].(float64); n != 1 {
		t.Fatalf("removedHooks = %v", res)
	}
	if want := "{\n  \"model\": \"opus\"\n}\n"; e.settings() != want {
		t.Fatalf("settings:\n%s", e.settings())
	}
}
