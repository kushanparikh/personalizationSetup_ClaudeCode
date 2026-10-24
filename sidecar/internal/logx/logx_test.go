package logx

import (
	"os"
	"strings"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

func TestRotateOverOneMiB(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := paths.LogPath()
	if err := os.MkdirAll(paths.LogDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, 1<<20+1), 0o644); err != nil {
		t.Fatal(err)
	}
	Open().Info("hello fresh")
	old, err := os.Stat(p + ".1")
	if err != nil || old.Size() != 1<<20+1 {
		t.Fatalf("pcs.log.1 missing or wrong size: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(b), "hello fresh") || len(b) > 1024 {
		t.Fatalf("fresh log wrong: %v %q", err, b)
	}
}

func TestOpenFallsBackToDiscard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A file where the log dir should be makes opening impossible.
	if err := os.MkdirAll(home+"/Library/Logs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.LogDir(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	Open().Info("must not panic")
}
