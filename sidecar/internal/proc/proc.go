// Package proc finds the terminal tab (tty) and terminal app a Claude Code
// session lives in, by walking up the process tree.
package proc

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Row is one line of `ps -A -o pid=,ppid=,tty=`.
type Row struct {
	PID, PPID int
	TTY       string
}

// ParsePS parses the output of `ps -A -o pid=,ppid=,tty=`, skipping malformed lines.
func ParsePS(out []byte) []Row {
	var rows []Row
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		rows = append(rows, Row{PID: pid, PPID: ppid, TTY: f[2]})
	}
	return rows
}

// FindTTY walks from start up through parents and returns /dev/<tty> of the
// first process with a real tty, or "" if none is found.
func FindTTY(rows []Row, start int) string {
	byPID := make(map[int]Row, len(rows))
	for _, r := range rows {
		byPID[r.PID] = r
	}
	seen := map[int]bool{}
	pid := start
	for pid > 1 && !seen[pid] {
		seen[pid] = true
		r, ok := byPID[pid]
		if !ok {
			return ""
		}
		if r.TTY != "" && r.TTY != "??" {
			if strings.HasPrefix(r.TTY, "/dev/") {
				return r.TTY
			}
			return "/dev/" + r.TTY
		}
		pid = r.PPID
	}
	return ""
}

// SessionTTY returns the tty of the current process tree using a single ps call.
func SessionTTY() string {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,tty=").Output()
	if err != nil {
		return ""
	}
	return FindTTY(ParsePS(out), os.Getpid())
}

// TerminalBundleID returns the bundle ID of the terminal app that launched the session.
func TerminalBundleID(getenv func(string) string) string {
	if id := getenv("__CFBundleIdentifier"); id != "" {
		return id
	}
	switch getenv("TERM_PROGRAM") {
	case "Apple_Terminal":
		return "com.apple.Terminal"
	case "iTerm.app":
		return "com.googlecode.iterm2"
	case "vscode":
		return "com.microsoft.VSCode"
	case "ghostty":
		return "com.mitchellh.ghostty"
	}
	return ""
}
