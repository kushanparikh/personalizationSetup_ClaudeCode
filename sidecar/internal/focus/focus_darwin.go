package focus

import (
	"os/exec"
	"strings"
)

// tabScript selects the Terminal.app tab whose tty is argv[1].
const tabScript = `on run argv
  set targetTTY to item 1 of argv
  tell application "Terminal"
    repeat with w in windows
      repeat with t in tabs of w
        if tty of t is targetTTY then
          set selected of t to true
          set index of w to 1
          activate
          return "found"
        end if
      end repeat
    end repeat
  end tell
  return "notfound"
end run`

// ExecRunner is the production Runner: it runs the command and returns trimmed stdout.
func ExecRunner(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}
