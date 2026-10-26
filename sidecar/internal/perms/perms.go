// Package perms describes the macOS permissions the app can need.
package perms

import "github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"

// ID names a permission.
type ID string

const (
	AutomationTerminal ID = "automation-terminal"
)

// Status is the state of a permission.
type Status string

const (
	Granted  Status = "granted"
	Denied   Status = "denied"
	NotAsked Status = "not-asked"
	Unknown  Status = "unknown"
)

// Info describes one permission: what the user sees, and how to check and
// request it. Adding a permission is one entry in registry.
type Info struct {
	ID          ID
	Title       string
	Steps       string
	SettingsURL string
	// Askable is true when the app can trigger the macOS prompt itself.
	Askable bool
	// UnknownHint is shown when the status is unknown.
	UnknownHint string
	// Check reports the status without prompting.
	Check func() Status
	// Request asks macOS for the permission; it may block on the prompt.
	Request func() Status
}

// terminalBundleID is the app whose Automation permission the app needs.
const terminalBundleID = "com.apple.Terminal"

var registry = []Info{
	{
		ID:          AutomationTerminal,
		Title:       "Control Terminal",
		Steps:       "Open System Settings → Privacy & Security → Automation → " + paths.AppName + ", and turn on Terminal.",
		SettingsURL: "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation",
		Askable:     true,
		UnknownHint: "Couldn't check this yet. Terminal may not be running. Click Ask for permission to check again.",
		Check:       func() Status { return CheckAutomation(terminalBundleID) },
		Request:     func() Status { return RequestAutomation(terminalBundleID) },
	},
}

// All returns every permission the app knows about.
func All() []Info { return append([]Info(nil), registry...) }

// SetRegistryForTest replaces the registry (so tests never call real TCC) and
// returns a function that restores it.
func SetRegistryForTest(r []Info) (restore func()) {
	old := registry
	registry = r
	return func() { registry = old }
}

// Lookup returns the Info for id; an unknown id yields an Info with only ID set.
func Lookup(id ID) Info {
	for _, i := range registry {
		if i.ID == id {
			return i
		}
	}
	return Info{ID: id}
}

// StatusFromOSStatus maps an AEDeterminePermissionToAutomateTarget result to a
// Status. Anything unrecognised, including -600 (target app not running), is Unknown.
func StatusFromOSStatus(code int) Status {
	switch code {
	case 0:
		return Granted
	case -1743:
		return Denied
	case -1744:
		return NotAsked
	}
	return Unknown
}
