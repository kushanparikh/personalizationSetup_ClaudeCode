package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := &Store{Version: 1, Hooks: map[hooktypes.Type]hooktypes.Answers{
		hooktypes.GitStatus: {UntrackedDirty: false},
		hooktypes.Finished:  {Sound: "Glass", ClickToFocus: true},
	}}
	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode %o want 600", info.Mode().Perm())
	}
	dinfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dinfo.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %o want 700", dinfo.Mode().Perm())
	}
}

func TestMissingIsEmpty(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if s.Version != 1 || s.Hooks == nil || len(s.Hooks) != 0 {
		t.Errorf("got %+v", s)
	}
}

func TestCorruptReturnsEmptyAndError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "hooks": {`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err == nil {
		t.Error("want error for corrupt file")
	}
	if s == nil || s.Version != 1 || s.Hooks == nil || len(s.Hooks) != 0 {
		t.Errorf("got %+v, want empty store", s)
	}
}

func TestZeroByteIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("zero-byte file: %v", err)
	}
	if s.Version != 1 || s.Hooks == nil || len(s.Hooks) != 0 {
		t.Errorf("got %+v", s)
	}
}

func TestLoadNormalizesVersionAndHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"version": 0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 1 || s.Hooks == nil {
		t.Errorf("got %+v", s)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := &Store{Version: 1, Hooks: map[hooktypes.Type]hooktypes.Answers{}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Errorf("unexpected dir contents: %v", entries)
	}
}
