// Package report builds the JSON snapshot the app shell renders: the app's
// hooks with their status, the other hooks, and the pending permissions.
package report

import (
	"bytes"
	"encoding/json"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/perms"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/settings"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/state"
)

// Hook statuses.
const (
	StatusOK      = "ok"
	StatusPending = "pending"
	StatusOff     = "off"
)

// Report is the full snapshot.
type Report struct {
	Hooks         []HookView      `json:"hooks"`
	Available     []hooktypes.Def `json:"available"`
	OtherHooks    []OtherHook     `json:"otherHooks"`
	Pending       []PendingItem   `json:"pending"`
	SettingsError string          `json:"settingsError"`
	LogPath       string          `json:"logPath"`
}

// HookView is one of the app's hook types as the user has set it up.
type HookView struct {
	Type        hooktypes.Type    `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Answers     hooktypes.Answers `json:"answers"`
	Status      string            `json:"status"`
	Needs       []Need            `json:"needs"`
}

// Need is one permission a hook needs and its current status.
type Need struct {
	ID     perms.ID     `json:"id"`
	Status perms.Status `json:"status"`
}

// OtherHook is a hook entry the app doesn't own; it is shown read-only.
type OtherHook struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Command string `json:"command"`
}

// PendingItem is a permission that an enabled hook needs and doesn't have.
type PendingItem struct {
	perms.Info
	Status  perms.Status
	Affects []string // hook Names
}

// MarshalJSON writes PendingItem with camelCase keys and no HTML escaping.
func (p PendingItem) MarshalJSON() ([]byte, error) {
	v := struct {
		ID          perms.ID     `json:"id"`
		Title       string       `json:"title"`
		Steps       string       `json:"steps"`
		SettingsURL string       `json:"settingsUrl"`
		Status      perms.Status `json:"status"`
		Affects     []string     `json:"affects"`
	}{p.ID, p.Title, p.Steps, p.SettingsURL, p.Status, p.Affects}
	if v.Affects == nil {
		v.Affects = []string{}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// CurrentAnswers returns the answers for t and whether it has owned entries.
// Answers come from the owned entries if there are any (for input-needed, the
// notification entry, since moved-on entries carry none), else from the
// stored answers, else the type's defaults.
func CurrentAnswers(f *settings.File, st *state.Store, t hooktypes.Type) (a hooktypes.Answers, enabled bool) {
	fromEntry := false
	if f != nil {
		for _, e := range f.Hooks() {
			if !e.Owned || e.Parsed.Type != t {
				continue
			}
			enabled = true
			if t != hooktypes.InputNeeded || e.Parsed.Sub == "notification" {
				a, fromEntry = e.Parsed.Answers, true
				break
			}
		}
	}
	if fromEntry {
		return a, enabled
	}
	if st != nil {
		if s, ok := st.Hooks[t]; ok {
			return s, enabled
		}
	}
	d, _ := hooktypes.Get(t)
	return d.Defaults, enabled
}

// Build assembles the report. have maps each permission to its status; a
// missing permission is unknown. If loadErr is set, only SettingsError and
// LogPath are filled.
func Build(f *settings.File, loadErr error, st *state.Store, have map[perms.ID]perms.Status) Report {
	r := Report{
		Hooks:      []HookView{},
		Available:  []hooktypes.Def{},
		OtherHooks: []OtherHook{},
		Pending:    []PendingItem{},
		LogPath:    paths.LogPath(),
	}
	if loadErr != nil || f == nil {
		if loadErr != nil {
			r.SettingsError = loadErr.Error()
		}
		return r
	}
	status := func(id perms.ID) perms.Status {
		if s, ok := have[id]; ok && s != "" {
			return s
		}
		return perms.Unknown
	}

	pendingAt := map[perms.ID]int{}
	for _, d := range hooktypes.All() {
		a, enabled := CurrentAnswers(f, st, d.Type)
		stored := false
		if st != nil {
			_, stored = st.Hooks[d.Type]
		}
		if !enabled && !stored {
			r.Available = append(r.Available, d)
			continue
		}
		v := HookView{Type: d.Type, Name: d.Name, Description: d.Description,
			Enabled: enabled, Answers: a, Status: StatusOff, Needs: []Need{}}
		for _, id := range hooktypes.Needs(d.Type, a) {
			v.Needs = append(v.Needs, Need{ID: id, Status: status(id)})
		}
		if enabled {
			v.Status = StatusOK
			for _, n := range v.Needs {
				if n.Status == perms.Granted {
					continue
				}
				v.Status = StatusPending
				i, ok := pendingAt[n.ID]
				if !ok {
					i = len(r.Pending)
					pendingAt[n.ID] = i
					r.Pending = append(r.Pending, PendingItem{Info: perms.Lookup(n.ID), Status: n.Status})
				}
				r.Pending[i].Affects = append(r.Pending[i].Affects, d.Name)
			}
		}
		r.Hooks = append(r.Hooks, v)
	}

	for _, e := range f.Hooks() {
		if !e.Owned {
			r.OtherHooks = append(r.OtherHooks, OtherHook{Event: e.Event, Matcher: e.Matcher, Command: e.Command})
		}
	}
	return r
}
