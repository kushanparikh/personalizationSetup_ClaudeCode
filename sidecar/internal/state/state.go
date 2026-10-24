// Package state stores each hook type's answers in ~/.pcs/state.json, so a
// hook that was turned off can be restored with the same answers. settings.json
// stays the source of truth; a damaged state file never blocks the app.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
)

const currentVersion = 1

// Store is the contents of state.json.
type Store struct {
	Version int                                  `json:"version"`
	Hooks   map[hooktypes.Type]hooktypes.Answers `json:"hooks"`
}

func empty() *Store {
	return &Store{Version: currentVersion, Hooks: map[hooktypes.Type]hooktypes.Answers{}}
}

// Load reads the store at path. A missing or zero-byte file is an empty store.
// A corrupt file gives an empty store and a non-nil error for the caller to log.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return empty(), fmt.Errorf("read state: %w", err)
	}
	if len(data) == 0 {
		return empty(), nil
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return empty(), fmt.Errorf("parse state %s: %w", path, err)
	}
	if s.Version == 0 {
		s.Version = currentVersion
	}
	if s.Hooks == nil {
		s.Hooks = map[hooktypes.Type]hooktypes.Answers{}
	}
	return &s, nil
}

// Save writes the store to path atomically: a temp file in the same directory
// is fsynced and renamed over path. The file is 0600 and its directory 0700.
func (s *Store) Save(path string) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	out := Store{Version: s.Version, Hooks: s.Hooks}
	if out.Version == 0 {
		out.Version = currentVersion
	}
	if out.Hooks == nil {
		out.Hooks = map[hooktypes.Type]hooktypes.Answers{}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temp state: %w", err)
	}
	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp state: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp state: %w", err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}
