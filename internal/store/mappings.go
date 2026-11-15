package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
)

// Mappings bind directories to profiles: `cswitch run` in a mapped directory
// (or below it) starts Claude Code as that profile. Keys are absolute,
// cleaned paths; the nearest mapped ancestor wins.
type Mappings map[string]string

func mappingsPath() string { return filepath.Join(Home(), "mappings.json") }

// LoadMappings returns the saved mappings (empty if none).
func LoadMappings() Mappings {
	m := Mappings{}
	if data, err := os.ReadFile(mappingsPath()); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

// SaveMappings writes the mappings.
func SaveMappings(m Mappings) error {
	if err := os.MkdirAll(Home(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(mappingsPath(), append(data, '\n'), 0o600)
}

// CleanDir makes dir absolute and clean, resolving symlinks when it exists,
// so the same directory always maps to the same key.
func CleanDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Clean(abs), nil
}

// Lookup returns the profile mapped to dir or its nearest mapped ancestor,
// and the mapped directory.
func (m Mappings) Lookup(dir string) (name, mappedDir string) {
	for d := dir; ; d = filepath.Dir(d) {
		if n, ok := m[d]; ok {
			return n, d
		}
		if parent := filepath.Dir(d); parent == d {
			return "", ""
		}
	}
}

// Dirs returns the mapped directories, sorted.
func (m Mappings) Dirs() []string {
	dirs := make([]string, 0, len(m))
	for d := range m {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// renameMappings points mappings of oldName at newName, or drops them when
// newName is "" (the profile was removed).
func renameMappings(oldName, newName string) {
	m := LoadMappings()
	changed := false
	for d, n := range m {
		if n == oldName {
			if newName == "" {
				delete(m, d)
			} else {
				m[d] = newName
			}
			changed = true
		}
	}
	if changed {
		_ = SaveMappings(m)
	}
}
