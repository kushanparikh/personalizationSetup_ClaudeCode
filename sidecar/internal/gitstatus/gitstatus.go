// Package gitstatus builds the one-line git summary shown at session start.
package gitstatus

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const notARepo = "📁 Not a git repository"

// run executes git -C cwd with optional locks disabled and returns trimmed-right stdout.
func run(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

// Summary returns the git status line for cwd. Outside a repository it returns
// the "Not a git repository" line and a nil error; it errors only when git is
// not installed. When untrackedDirty is false, untracked files neither make the
// repo dirty nor appear in the dirty text.
func Summary(cwd string, untrackedDirty bool) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not found: %w", err)
	}
	inside, err := run(cwd, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return notARepo, nil
	}
	top, err := run(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return notARepo, nil
	}
	repo := filepath.Base(strings.TrimSpace(top))

	branch, err := run(cwd, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		sha, _ := run(cwd, "rev-parse", "--short", "HEAD")
		branch = "detached@" + strings.TrimSpace(sha)
	}
	branch = strings.TrimSpace(branch)

	status, err := run(cwd, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	var staged, unstaged, untracked int
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 2 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "??"):
			untracked++
			continue
		case strings.ContainsRune("MADRC", rune(line[0])):
			staged++
		}
		if line[1] == 'M' || line[1] == 'D' {
			unstaged++
		}
	}

	state := "✅ clean"
	if staged+unstaged > 0 || (untrackedDirty && untracked > 0) {
		if untrackedDirty {
			state = fmt.Sprintf("⚠️ dirty (staged: %d, unstaged: %d, untracked: %d)", staged, unstaged, untracked)
		} else {
			state = fmt.Sprintf("⚠️ dirty (staged: %d, unstaged: %d)", staged, unstaged)
		}
	}
	return fmt.Sprintf("📁 %s  🌿 %s  %s", repo, branch, state), nil
}
