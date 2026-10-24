// Package perms describes the macOS permissions the app can need.
package perms

// ID names a permission.
type ID string

const (
	Notifications      ID = "notifications"
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

// Info is the user-facing description of a permission.
type Info struct {
	ID          ID
	Title       string
	Steps       string
	SettingsURL string
}

// Lookup returns the Info for id; an unknown id yields an Info with only ID set.
func Lookup(id ID) Info {
	switch id {
	case Notifications:
		return Info{
			ID:          id,
			Title:       "Notifications",
			Steps:       "Open System Settings → Notifications → Personalization Setup, turn on Allow Notifications, and choose the Temporary style.",
			SettingsURL: "x-apple.systempreferences:com.apple.Notifications-Settings.extension",
		}
	case AutomationTerminal:
		return Info{
			ID:          id,
			Title:       "Control Terminal",
			Steps:       "Open System Settings → Privacy & Security → Automation → Personalization Setup, and turn on Terminal.",
			SettingsURL: "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation",
		}
	}
	return Info{ID: id}
}
