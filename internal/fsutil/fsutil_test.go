package fsutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")

	if err := AtomicWrite(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Errorf("new file mode = %v, want 0600", fi.Mode().Perm())
	}

	os.Chmod(p, 0o640)
	if err := AtomicWrite(p, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "two" {
		t.Errorf("content = %q", got)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o640 {
		t.Errorf("existing mode not kept: %v", fi.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".f.cswitch-*")); len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}

func TestAtomicWriteMissingDir(t *testing.T) {
	if err := AtomicWrite(filepath.Join(t.TempDir(), "nope", "f"), []byte("x"), 0o600); err == nil {
		t.Error("expected error for missing parent directory")
	}
}

func TestCopyFilePreservesMtime(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "sub", "dst")
	write(t, src, "data")
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	os.Chtimes(src, old, old)

	if err := CopyFile(src, dst, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dst); got != "data" {
		t.Errorf("content = %q", got)
	}
	fi, _ := os.Stat(dst)
	if !fi.ModTime().Equal(old) {
		t.Errorf("mtime = %v, want %v", fi.ModTime(), old)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := CopyFile(filepath.Join(dir, "none"), filepath.Join(dir, "dst"), 0o600); !os.IsNotExist(err) {
		t.Errorf("err = %v, want not-exist", err)
	}
}

func TestCopyPathTreeReplacesDestination(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	write(t, filepath.Join(src, "a"), "A")
	write(t, filepath.Join(src, "deep", "b"), "B")
	os.Symlink("/etc/hosts", filepath.Join(src, "link"))
	write(t, filepath.Join(dst, "stale"), "old")

	if err := CopyPath(src, dst); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dst, "a")) != "A" || read(t, filepath.Join(dst, "deep", "b")) != "B" {
		t.Error("tree not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "stale")); !os.IsNotExist(err) {
		t.Error("existing destination not replaced")
	}
	if _, err := os.Lstat(filepath.Join(dst, "link")); !os.IsNotExist(err) {
		t.Error("symlink should be skipped")
	}
}

func TestCopyPathSingleFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "f"), "x")
	if err := CopyPath(filepath.Join(dir, "f"), filepath.Join(dir, "g")); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "g")) != "x" {
		t.Error("file not copied")
	}
}

func TestBackupAndPrune(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, "backups")
	src := filepath.Join(dir, "config.json")
	write(t, src, "v0")

	if err := Backup(filepath.Join(dir, "missing"), backups); err != nil {
		t.Errorf("missing source should be ignored: %v", err)
	}
	// Unrelated file with a similar name must never be pruned.
	write(t, filepath.Join(backups, "config.json-journal.20000101-000000.000"), "other")

	for i := 0; i < keepBackups+3; i++ {
		write(t, src, fmt.Sprintf("v%d", i))
		if err := Backup(src, backups); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // distinct millisecond timestamps
	}
	got, _ := filepath.Glob(filepath.Join(backups, "config.json.2*"))
	if len(got) != keepBackups {
		t.Fatalf("kept %d backups, want %d", len(got), keepBackups)
	}
	// The newest backup holds the last version.
	newest := got[len(got)-1]
	if read(t, newest) != fmt.Sprintf("v%d", keepBackups+2) {
		t.Errorf("newest backup = %q", read(t, newest))
	}
	if _, err := os.Stat(filepath.Join(backups, "config.json-journal.20000101-000000.000")); err != nil {
		t.Error("pruning removed an unrelated file")
	}
	if read(t, src) != fmt.Sprintf("v%d", keepBackups+2) {
		t.Error("Backup modified the source")
	}
}

func TestMoveToBackup(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, "backups")
	tree := filepath.Join(dir, "Local Storage")
	write(t, filepath.Join(tree, "leveldb", "x"), "data")

	if err := MoveToBackup(filepath.Join(dir, "missing"), backups); err != nil {
		t.Errorf("missing path should be ignored: %v", err)
	}
	if err := MoveToBackup(tree, backups); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tree); !os.IsNotExist(err) {
		t.Error("source still exists")
	}
	moved, _ := filepath.Glob(filepath.Join(backups, "Local Storage.2*"))
	if len(moved) != 1 || read(t, filepath.Join(moved[0], "leveldb", "x")) != "data" {
		t.Errorf("moved = %v", moved)
	}

	// Directories are pruned too.
	for i := 0; i < keepBackups+2; i++ {
		write(t, filepath.Join(tree, "f"), "x")
		if err := MoveToBackup(tree, backups); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if moved, _ := filepath.Glob(filepath.Join(backups, "Local Storage.2*")); len(moved) != keepBackups {
		t.Errorf("kept %d directory backups, want %d", len(moved), keepBackups)
	}
}

func TestJSONObjectRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	write(t, p, `{"b":{"nested":[1,2]},"a":"x<y&z","n":null}`)

	m, err := ReadJSONObject(p)
	if err != nil {
		t.Fatal(err)
	}
	m["c"] = json.RawMessage(`true`)
	if err := WriteJSONObject(p, m); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(read(t, p)), &got); err != nil {
		t.Fatal(err)
	}
	if got["a"] != "x<y&z" || got["c"] != true || got["n"] != nil {
		t.Errorf("round trip = %v", got)
	}
	if nested := got["b"].(map[string]any)["nested"].([]any); len(nested) != 2 {
		t.Errorf("nested = %v", nested)
	}
	if !strings.HasSuffix(read(t, p), "}\n") {
		t.Error("missing trailing newline")
	}
}

func TestReadJSONObjectRejectsNonObject(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"array": `[1]`, "garbage": `{nope`, "string": `"x"`} {
		p := filepath.Join(dir, name)
		write(t, p, content)
		if _, err := ReadJSONObject(p); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := ReadJSONObject(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Errorf("missing file: %v", err)
	}
}

func TestWritesFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "claude.json")
	link := filepath.Join(dir, ".claude.json")
	write(t, real, "old")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := AtomicWrite(link, []byte("atomic"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("AtomicWrite replaced the symlink with a regular file")
	}
	if read(t, real) != "atomic" {
		t.Errorf("target = %q", read(t, real))
	}

	src := filepath.Join(dir, "src")
	write(t, src, "copied")
	if err := CopyFile(src, link, 0o600); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("CopyFile replaced the symlink with a regular file")
	}
	if read(t, real) != "copied" {
		t.Errorf("target = %q", read(t, real))
	}
}

func TestReadJSONObjectRejectsNull(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	write(t, p, "null")
	if m, err := ReadJSONObject(p); err == nil {
		t.Errorf("null accepted as object: %v", m)
	}
}
