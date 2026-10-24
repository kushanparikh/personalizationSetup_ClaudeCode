package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallIdempotent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bin", "pcs-hook")
	changed, err := Install(p, "/Applications/X.app/Contents/MacOS/pcs")
	if err != nil || !changed {
		t.Fatalf("first install: changed=%v err=%v", changed, err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `BIN="/Applications/X.app/Contents/MacOS/pcs"`) || !strings.HasPrefix(string(data), "#!/bin/sh\n") {
		t.Fatalf("script = %s", data)
	}
	changed, err = Install(p, "/Applications/X.app/Contents/MacOS/pcs")
	if err != nil || changed {
		t.Fatalf("second install: changed=%v err=%v", changed, err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
	changed, err = Install(p, "/other/pcs")
	if err != nil || !changed {
		t.Fatalf("new bin: changed=%v err=%v", changed, err)
	}
}

func TestLauncherRejectsUnsafePath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pcs-hook")
	for _, bin := range []string{`/a"b`, "/a$HOME", "/a`id`", `/a\b`} {
		if _, err := Install(p, bin); err == nil {
			t.Errorf("Install(%q) accepted an unsafe path", bin)
		}
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("script written for unsafe path: %v", err)
	}
}

func TestLauncherMissingAppExitsZero(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pcs-hook")
	if _, err := Install(p, filepath.Join(t.TempDir(), "missing", "pcs")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(p, "hook", "stop")
	cmd.Stdin = strings.NewReader(`{"x":1}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit: %v (%s)", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("output = %q", out)
	}
}

func TestLauncherExecsBin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "my app", "pcs")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "pcs-hook")
	if _, err := Install(p, bin); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(p, "hook", "stop").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "hook stop" {
		t.Fatalf("output = %q", out)
	}
}
