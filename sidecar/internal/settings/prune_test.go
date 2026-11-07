package settings

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPruneOrderWithSuffixes(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"settings.pcs-backup-20261024T101010.000Z.json",
		"settings.pcs-backup-20261024T101010.123Z.json",
		"settings.pcs-backup-20261024T101010.123Z-1.json",
		"settings.pcs-backup-20261024T101010.123Z-2.json",
		"settings.pcs-backup-20261024T101010.123Z-10.json",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := prune(dir, 2); err != nil {
		t.Fatal(err)
	}
	var got []string
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		got = append(got, e.Name())
	}
	want := []string{names[4], names[3]} // ReadDir sorts by name: "-10" < "-2"
	if !slices.Equal(got, want) {
		t.Fatalf("kept %v, want %v", got, want)
	}
}
