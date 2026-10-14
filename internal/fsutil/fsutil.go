// Package fsutil holds small file helpers shared by the other packages.
package fsutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// resolve follows symlinks so writes land in the real file instead of
// replacing the link (dotfile managers often symlink ~/.claude.json).
// Paths that do not exist yet are returned unchanged.
func resolve(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	return real, err
}

// AtomicWrite writes data to a temp file next to path and renames it into place.
// The existing file's permissions are kept; perm is used when path does not
// exist. If path is a symlink, its target is written and the link is kept.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	path, err := resolve(path)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".cswitch-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// CopyFile copies src to dst (atomically) and preserves the modification time.
// If dst is a symlink, its target is written and the link is kept.
func CopyFile(src, dst string, perm os.FileMode) error {
	dst, err := resolve(dst)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".cswitch-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

const keepBackups = 10

// Backup copies path into dir, stamped with the current time, and prunes all
// but the newest backups of that file. Missing source files are ignored.
func Backup(path, dir string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	base := filepath.Base(path)
	name := fmt.Sprintf("%s.%s", base, time.Now().Format("20060102-150405.000"))
	if err := CopyFile(path, filepath.Join(dir, name), 0o600); err != nil {
		return err
	}
	prune(dir, base)
	return nil
}

// ReadJSONObject reads a JSON object file as a map of raw values, so that
// unknown keys survive a read-modify-write cycle untouched.
func ReadJSONObject(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: not a JSON object: %w", path, err)
	}
	if m == nil { // the file is literally "null"
		return nil, fmt.Errorf("%s: not a JSON object", path)
	}
	return m, nil
}

// WriteJSONObject writes m to path atomically with two-space indentation.
func WriteJSONObject(path string, m map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, append(data, '\n'), 0o600)
}

// CopyPath copies a file or directory tree from src to dst, replacing dst.
// File modification times are preserved.
func CopyPath(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if !fi.IsDir() {
		return CopyFile(src, dst, 0o600)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o700)
		case d.Type().IsRegular():
			return CopyFile(path, target, 0o600)
		default:
			return nil // sockets, symlinks and lock files are not worth carrying over
		}
	})
}

// MoveToBackup moves a file or directory into dir under a timestamped name
// and prunes older backups of it. A missing path is ignored.
func MoveToBackup(path, dir string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	base := filepath.Base(path)
	dst := filepath.Join(dir, fmt.Sprintf("%s.%s", base, time.Now().Format("20060102-150405.000")))
	if err := os.Rename(path, dst); err != nil {
		// Different volume: fall back to copy + remove.
		if err := CopyPath(path, dst); err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	prune(dir, base)
	return nil
}

// prune keeps only the newest keepBackups backups of base in dir;
// the timestamped names sort chronologically.
func prune(dir, base string) {
	old, _ := filepath.Glob(filepath.Join(dir, base+".2*"))
	sort.Strings(old)
	for len(old) > keepBackups {
		os.RemoveAll(old[0])
		old = old[1:]
	}
}
