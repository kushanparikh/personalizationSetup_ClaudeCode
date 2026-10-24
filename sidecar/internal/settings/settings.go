// Package settings edits ~/.claude/settings.json. It only ever changes the
// app's own hook entries, keeps everything else as it is, and refuses to
// write while the file is invalid JSON.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/hooktypes"
	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/jsonx"
)

// File is a loaded settings.json.
type File struct {
	Path string
	root *jsonx.Object
}

// InvalidError reports a settings.json that isn't valid JSON.
type InvalidError struct {
	Path      string
	Line, Col int
	Msg       string
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("%s isn't valid JSON (line %d, column %d): %s. Fix it, then reopen. Nothing will be changed until then.",
		e.Path, e.Line, e.Col, e.Msg)
}

// parse decodes data; empty data is an empty object.
func parse(path string, data []byte) (*jsonx.Object, error) {
	if len(data) == 0 {
		return jsonx.NewObject(), nil
	}
	o, err := jsonx.Parse(data)
	if err != nil {
		var se *jsonx.SyntaxError
		if errors.As(err, &se) {
			return nil, &InvalidError{Path: path, Line: se.Line, Col: se.Col, Msg: se.Msg}
		}
		return nil, &InvalidError{Path: path, Line: 1, Col: 1, Msg: err.Error()}
	}
	return o, nil
}

// readTarget reads path; a missing file gives nil data.
func readTarget(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// Load reads path. A missing or empty file gives an empty object; invalid
// JSON gives *InvalidError.
func Load(path string) (*File, error) {
	data, err := readTarget(path)
	if err != nil {
		return nil, err
	}
	root, err := parse(path, data)
	if err != nil {
		return nil, err
	}
	return &File{Path: path, root: root}, nil
}

// HookEntry is one inner hook entry, in file order.
type HookEntry struct {
	Event, Matcher, Command string
	Owned                   bool
	Parsed                  hooktypes.Parsed
}

// commandOf returns the command of a type:"command" inner entry.
func commandOf(inner any) (string, bool) {
	o, ok := inner.(*jsonx.Object)
	if !ok {
		return "", false
	}
	if typ, _ := o.Vals["type"].(string); typ != "command" {
		return "", false
	}
	c, ok := o.Vals["command"].(string)
	return c, ok
}

// owned reports whether inner is an app-owned entry, and parses it.
func owned(inner any) (hooktypes.Parsed, bool) {
	c, ok := commandOf(inner)
	if !ok {
		return hooktypes.Parsed{}, false
	}
	return hooktypes.ParseCommand(c)
}

// hooksObj returns the root "hooks" object. ok is false if hooks exists but
// isn't an object, which the app must leave alone.
func (f *File) hooksObj() (h *jsonx.Object, ok bool) {
	v, exists := f.root.Get("hooks")
	if !exists {
		return nil, true
	}
	h, ok = v.(*jsonx.Object)
	return h, ok
}

// Hooks lists every inner hook entry in file order. Only type:"command"
// entries have a Command. Malformed shapes are skipped.
func (f *File) Hooks() []HookEntry {
	h, _ := f.hooksObj()
	if h == nil {
		return nil
	}
	var out []HookEntry
	for _, ev := range h.Keys {
		groups, _ := h.Vals[ev].([]any)
		for _, g := range groups {
			gobj, ok := g.(*jsonx.Object)
			if !ok {
				continue
			}
			matcher, _ := gobj.Vals["matcher"].(string)
			inners, _ := gobj.Vals["hooks"].([]any)
			for _, in := range inners {
				if _, ok := in.(*jsonx.Object); !ok {
					continue
				}
				e := HookEntry{Event: ev, Matcher: matcher}
				e.Command, _ = commandOf(in)
				e.Parsed, e.Owned = owned(in)
				out = append(out, e)
			}
		}
	}
	return out
}

// remove deletes owned inner entries matching match. Groups, event arrays and
// hooks emptied by this are deleted, except event arrays named in keep (and
// so hooks) which stay in place as [] for the caller to refill.
func (f *File) remove(match func(hooktypes.Parsed) bool, keep map[string]bool) int {
	h, ok := f.hooksObj()
	if !ok || h == nil {
		return 0
	}
	total := 0
	for _, ev := range slices.Clone(h.Keys) {
		groups, ok := h.Vals[ev].([]any)
		if !ok {
			continue
		}
		removedGroup := false
		var keptGroups []any
		for _, g := range groups {
			gobj, ok := g.(*jsonx.Object)
			inners, isList := []any(nil), false
			if ok {
				inners, isList = gobj.Vals["hooks"].([]any)
			}
			if !isList {
				keptGroups = append(keptGroups, g)
				continue
			}
			var keptInners []any
			for _, in := range inners {
				if p, ok := owned(in); ok && match(p) {
					total++
					continue
				}
				keptInners = append(keptInners, in)
			}
			switch {
			case len(keptInners) == len(inners):
				keptGroups = append(keptGroups, g)
			case len(keptInners) == 0:
				removedGroup = true
			default:
				gobj.Set("hooks", keptInners)
				keptGroups = append(keptGroups, g)
			}
		}
		if !removedGroup {
			continue
		}
		if len(keptGroups) == 0 && !keep[ev] {
			h.Delete(ev)
			continue
		}
		if keptGroups == nil {
			keptGroups = []any{}
		}
		h.Set(ev, keptGroups)
	}
	if total > 0 && len(h.Keys) == 0 {
		f.root.Delete("hooks")
	}
	return total
}

func ofType(t hooktypes.Type) func(hooktypes.Parsed) bool {
	return func(p hooktypes.Parsed) bool { return p.Type == t }
}

// RemoveOwned removes the app's entries of type t and returns how many.
func (f *File) RemoveOwned(t hooktypes.Type) int {
	return f.remove(ofType(t), nil)
}

// RemoveAllOwned removes every app-owned entry and returns how many.
func (f *File) RemoveAllOwned() int {
	return f.remove(func(hooktypes.Parsed) bool { return true }, nil)
}

// SetOwned replaces the app's entries of type t with es, one new matcher
// group per entry. Entries that aren't app-owned commands of type t are
// skipped so nothing unremovable is ever written. If hooks, or an event es
// targets, has a malformed shape, SetOwned changes nothing at all.
func (f *File) SetOwned(t hooktypes.Type, es []hooktypes.Entry) {
	h, ok := f.hooksObj()
	if !ok {
		return
	}
	var add []hooktypes.Entry
	keep := map[string]bool{}
	for _, e := range es {
		if p, ok := hooktypes.ParseCommand(e.Command); !ok || p.Type != t {
			continue
		}
		if h != nil {
			if v, exists := h.Get(e.Event); exists {
				if _, isList := v.([]any); !isList {
					return
				}
			}
		}
		add = append(add, e)
		keep[e.Event] = true
	}
	f.remove(ofType(t), keep)

	if h == nil {
		if len(add) == 0 {
			return
		}
		h = jsonx.NewObject()
		f.root.Set("hooks", h)
	}
	for _, e := range add {
		inner := jsonx.NewObject()
		inner.Set("type", "command")
		inner.Set("command", e.Command)
		inner.Set("timeout", jsonNumber(e.Timeout))
		group := jsonx.NewObject()
		if e.Matcher != "" {
			group.Set("matcher", e.Matcher)
		}
		group.Set("hooks", []any{inner})
		groups, _ := h.Vals[e.Event].([]any)
		h.Set(e.Event, append(groups, group))
	}
}

func jsonNumber(n int) json.Number { return json.Number(strconv.Itoa(n)) }

// SaveOptions controls backups. Keep <= 0 means 5.
type SaveOptions struct {
	BackupDir string
	Keep      int
}

const (
	backupPrefix = "settings.pcs-backup-"
	backupLayout = "20060102T150405.000Z"
)

var backupName = regexp.MustCompile(`^settings\.pcs-backup-(\d{8}T\d{6}\.\d{3}Z)(?:-(\d+))?\.json$`)

// resolve returns the file to write: Path with symlinks followed, or Path
// itself if it doesn't exist. A dangling symlink is refused rather than
// replaced by a regular file.
func (f *File) resolve() (string, error) {
	target, err := filepath.EvalSymlinks(f.Path)
	if err == nil {
		return target, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if st, lerr := os.Lstat(f.Path); lerr == nil && st.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symlink to a missing file; not writing", f.Path)
	}
	return f.Path, nil
}

// Save writes the file: it re-reads and validates the target, backs it up,
// checks the new content parses, and replaces the target atomically. It
// returns the backup path, or "" if there was nothing to back up.
func (f *File) Save(o SaveOptions) (backupPath string, err error) {
	if o.BackupDir == "" {
		return "", errors.New("settings: no backup directory")
	}
	if o.Keep <= 0 {
		o.Keep = 5
	}
	target, err := f.resolve()
	if err != nil {
		return "", err
	}
	current, err := readTarget(target)
	if err != nil {
		return "", err
	}
	if _, err := parse(f.Path, current); err != nil {
		return "", err
	}
	out := jsonx.Marshal(f.root)
	if _, err := jsonx.Parse(out); err != nil {
		return "", fmt.Errorf("settings: refusing to write output that doesn't parse: %w", err)
	}

	mode := fs.FileMode(0o600)
	if st, err := os.Stat(target); err == nil {
		mode = st.Mode().Perm()
	}
	if len(current) > 0 {
		if backupPath, err = backup(o, current); err != nil {
			return "", err
		}
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return backupPath, err
	}
	return backupPath, writeAtomic(target, out, mode)
}

func backup(o SaveOptions, data []byte) (string, error) {
	if err := os.MkdirAll(o.BackupDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format(backupLayout)
	var path string
	for i := 0; ; i++ {
		name := backupPrefix + stamp + ".json"
		if i > 0 {
			name = backupPrefix + stamp + "-" + strconv.Itoa(i) + ".json"
		}
		path = filepath.Join(o.BackupDir, name)
		fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := fh.Write(data)
		if serr := fh.Sync(); werr == nil {
			werr = serr
		}
		if cerr := fh.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(path)
			return "", werr
		}
		break
	}
	return path, prune(o.BackupDir, o.Keep)
}

// prune deletes all but the keep newest backups. Only names matching the
// backup pattern are considered.
func prune(dir string, keep int) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type bk struct {
		name, stamp string
		n           int
	}
	var bks []bk
	for _, e := range ents {
		m := backupName.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		n := 0
		if m[2] != "" {
			if n, err = strconv.Atoi(m[2]); err != nil {
				continue
			}
		}
		bks = append(bks, bk{e.Name(), m[1], n})
	}
	slices.SortFunc(bks, func(a, b bk) int {
		if a.stamp != b.stamp {
			if a.stamp < b.stamp {
				return -1
			}
			return 1
		}
		return a.n - b.n
	})
	for len(bks) > keep {
		if err := os.Remove(filepath.Join(dir, bks[0].name)); err != nil {
			return err
		}
		bks = bks[1:]
	}
	return nil
}

// writeAtomic writes data to a temp file beside path with mode, fsyncs it
// and renames it over path.
func writeAtomic(path string, data []byte, mode fs.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".settings.json.pcs-tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if d, derr := os.Open(dir); derr == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
