package gitstatus

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.name", "Test")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func write(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		write(t, dir, n)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "init")
}

func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	os.Exit(m.Run())
}

func TestClean(t *testing.T) {
	dir := newRepo(t, "proj")
	commitFiles(t, dir, "a.txt")
	got, err := Summary(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "📁 proj  🌿 main  ✅ clean"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDirtyCounts(t *testing.T) {
	dir := newRepo(t, "proj")
	commitFiles(t, dir, "m1.txt", "m2.txt")
	write(t, dir, "new.txt")
	git(t, dir, "add", "new.txt")
	if err := os.WriteFile(filepath.Join(dir, "m1.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "m2.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "u1")
	write(t, dir, "u2")
	write(t, dir, "u3")
	got, err := Summary(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "📁 proj  🌿 main  ⚠️ dirty (staged: 1, unstaged: 2, untracked: 3)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got, _ = Summary(dir, false)
	if want := "📁 proj  🌿 main  ⚠️ dirty (staged: 1, unstaged: 2)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUntrackedClean(t *testing.T) {
	dir := newRepo(t, "proj")
	commitFiles(t, dir, "a.txt")
	write(t, dir, "u1")
	got, err := Summary(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "📁 proj  🌿 main  ✅ clean"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDetached(t *testing.T) {
	dir := newRepo(t, "proj")
	commitFiles(t, dir, "a.txt")
	sha := git(t, dir, "rev-parse", "--short", "HEAD")
	git(t, dir, "checkout", "--detach")
	got, err := Summary(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "📁 proj  🌿 detached@" + sha + "  ✅ clean"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNotARepo(t *testing.T) {
	got, err := Summary(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "📁 Not a git repository"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSummaryPathWithSpaces(t *testing.T) {
	dir := newRepo(t, "My Project")
	commitFiles(t, dir, "a.txt")
	got, err := Summary(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "📁 My Project  🌿 ") {
		t.Fatalf("got %q", got)
	}
}
