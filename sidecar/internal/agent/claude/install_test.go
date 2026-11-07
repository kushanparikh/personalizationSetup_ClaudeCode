package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
)

func TestInstallRejectsBadEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fin := Entries(hooktypes.Finished, hooktypes.Answers{Sound: "Glass"})
	in := Entries(hooktypes.InputNeeded, hooktypes.Answers{Sound: "Ping"})
	foreign := []hooktypes.Entry{{Event: "Stop", Command: "~/.claude/hooks/notify-done.sh"}}
	kimi := []hooktypes.Entry{{Event: "Stop", Command: hooktypes.CommandPrefix + "--agent kimi stop --sound Glass --focus"}}
	for name, es := range map[string][]hooktypes.Entry{
		"empty": nil, "mixed": append(fin[:1:1], in[0]), "foreign": foreign, "other agent": kimi,
	} {
		if err := (Adapter{}).Install(es); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestInstallThenUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"theme":"auto"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	es := Entries(hooktypes.Finished, hooktypes.Answers{Sound: "Glass", ClickToFocus: true})
	if err := (Adapter{}).Install(es); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "hook stop --sound Glass --focus") || !strings.Contains(string(b), `"theme": "auto"`) {
		t.Fatalf("after install:\n%s", b)
	}
	n, backup, err := (Adapter{}).Uninstall()
	if err != nil || n != len(es) || filepath.Dir(backup) != filepath.Dir(p) {
		t.Fatalf("uninstall: n=%d backup=%q err=%v", n, backup, err)
	}
	b, _ = os.ReadFile(p)
	if strings.Contains(string(b), "pcs-hook") || !strings.Contains(string(b), `"theme": "auto"`) {
		t.Fatalf("after uninstall:\n%s", b)
	}
}
