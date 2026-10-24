// Package focus brings a terminal tab or app to the front.
package focus

import "strings"

const terminalBundleID = "com.apple.Terminal"

// Runner runs a command and returns its trimmed stdout.
type Runner func(name string, args ...string) (string, error)

// Focus brings the terminal owning tty to the front. It returns "tab" when the
// exact Terminal.app tab was selected, "app" when only the app was brought
// forward, and "none" when nothing worked. It never reports errors.
func Focus(tty, bundleID string, run Runner) string {
	if bundleID == "" {
		bundleID = terminalBundleID
	}
	if bundleID == terminalBundleID && tty != "" {
		out, err := run("osascript", "-e", tabScript, tty)
		if err == nil && strings.TrimSpace(out) == "found" {
			return "tab"
		}
	}
	if _, err := run("open", "-b", bundleID); err != nil {
		return "none"
	}
	return "app"
}
