// Package desktop snapshots and restores Claude Desktop's login state and
// controls the app process.
//
// Desktop keeps its OAuth tokens in config.json under "oauth:*" keys, encrypted
// with Electron safeStorage (key in the "Claude Safe Storage" keychain item).
// The key is per machine, so the encrypted strings can be saved and restored
// byte for byte without decrypting them. Per-account web state (cookies and
// the claude.ai origin's storage) lives in the files listed in stateFiles.
package desktop

import (
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
)

// Dir is Claude Desktop's data directory (CSWITCH_DESKTOP_DIR overrides it).
func Dir() string {
	if d := os.Getenv("CSWITCH_DESKTOP_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Claude")
}

func configPath() string { return filepath.Join(Dir(), "config.json") }

const accountKey = "lastKnownAccountUuid"

// stateFiles are the per-account files and directories swapped on a switch:
// the cookie database and the web storage of the claude.ai origin, so the UI
// never shows one account's cached data while logged in as another.
var stateFiles = []string{
	"Cookies", "Cookies-journal",
	"Local Storage", "Session Storage", "IndexedDB", "WebStorage",
}

func isAuthKey(k string) bool { return strings.HasPrefix(k, "oauth:") || k == accountKey }

type notLoggedIn struct{}

func (notLoggedIn) Error() string { return i18n.T("err.desktop_not_logged_in") }

// ErrNotLoggedIn means config.json holds no OAuth tokens.
var ErrNotLoggedIn error = notLoggedIn{}

// Snapshot is Desktop's login state from config.json. AccountUUID is empty
// when tokens exist but the account cannot be identified.
type Snapshot struct {
	AccountUUID string                     `json:"accountUuid"`
	Keys        map[string]json.RawMessage `json:"keys"`
}

// Read returns the current login state, or ErrNotLoggedIn if there are no tokens.
func Read() (*Snapshot, error) {
	m, err := fsutil.ReadJSONObject(configPath())
	if os.IsNotExist(err) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	s := &Snapshot{Keys: map[string]json.RawMessage{}}
	hasToken := false
	for k, v := range m {
		if isAuthKey(k) {
			s.Keys[k] = v
			hasToken = hasToken || (strings.HasPrefix(k, "oauth:") && !emptyJSON(v))
		}
	}
	if !hasToken {
		return nil, ErrNotLoggedIn
	}
	if raw, ok := m[accountKey]; ok {
		_ = json.Unmarshal(raw, &s.AccountUUID)
	}
	return s, nil
}

// emptyJSON reports whether v carries no data: null, "", {} or []. Desktop may
// blank its token keys on logout or expiry while keeping lastKnownAccountUuid,
// and that state must not count as a login worth saving.
func emptyJSON(v json.RawMessage) bool {
	switch strings.TrimSpace(string(v)) {
	case "", "null", `""`, "{}", "[]":
		return true
	}
	return false
}

// CurrentAccount returns the account UUID Desktop is logged in as, or "" when
// it is logged out or the account is unknown.
func CurrentAccount() string {
	s, err := Read()
	if err != nil {
		return ""
	}
	return s.AccountUUID
}

// Restore replaces the auth keys in config.json with the snapshot's.
// A nil snapshot just removes them (local logout without revoking tokens).
func Restore(s *Snapshot, backupDir string) error {
	path := configPath()
	m, err := fsutil.ReadJSONObject(path)
	if os.IsNotExist(err) {
		m = map[string]json.RawMessage{}
	} else if err != nil {
		return err
	}
	if err := fsutil.Backup(path, backupDir); err != nil {
		return i18n.E("err.backup", path, err)
	}
	for k := range m {
		if isAuthKey(k) {
			delete(m, k)
		}
	}
	if s != nil {
		maps.Copy(m, s.Keys)
	}
	return fsutil.WriteJSONObject(path, m)
}

// SaveState copies Desktop's per-account files into dst. Entries missing from
// Desktop are removed from dst so a later restore reproduces the same set.
func SaveState(dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, f := range stateFiles {
		src := filepath.Join(Dir(), f)
		if _, err := os.Lstat(src); os.IsNotExist(err) {
			if err := os.RemoveAll(filepath.Join(dst, f)); err != nil {
				return err
			}
			continue
		}
		if err := fsutil.CopyPath(src, filepath.Join(dst, f)); err != nil {
			return err
		}
	}
	return nil
}

// RestoreState replaces Desktop's per-account files with the copies in src.
// The live entries are moved to backupDir first, so a stale cookie journal is
// never replayed onto another database. An empty src just clears them.
func RestoreState(src, backupDir string) error {
	for _, f := range stateFiles {
		if err := fsutil.MoveToBackup(filepath.Join(Dir(), f), backupDir); err != nil {
			return err
		}
	}
	if src == "" {
		return nil
	}
	for _, f := range stateFiles {
		saved := filepath.Join(src, f)
		if _, err := os.Lstat(saved); os.IsNotExist(err) {
			continue
		}
		if err := fsutil.CopyPath(saved, filepath.Join(Dir(), f)); err != nil {
			return err
		}
	}
	return nil
}

// Checkpoint is a copy of Desktop's login state taken before a change, so a
// failed switch can put back exactly what was there.
type Checkpoint struct{ dir string }

// staleCheckpointAge is how old a checkpoint must be before it counts as left
// over from an interrupted run rather than belonging to one in progress.
const staleCheckpointAge = 10 * time.Minute

// NewCheckpoint copies config.json and the per-account files into a fresh
// directory under parent. Checkpoints left behind by interrupted runs (they
// hold copies of tokens and cookies) are deleted first.
func NewCheckpoint(parent string) (*Checkpoint, error) {
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, err
	}
	removeStaleCheckpoints(parent)
	dir, err := os.MkdirTemp(parent, "checkpoint-")
	if err != nil {
		return nil, err
	}
	cp := &Checkpoint{dir: dir}
	if err := SaveState(filepath.Join(dir, "state")); err != nil {
		cp.Discard()
		return nil, err
	}
	if _, err := os.Stat(configPath()); err == nil {
		if err := fsutil.CopyFile(configPath(), filepath.Join(dir, "config.json"), 0o600); err != nil {
			cp.Discard()
			return nil, err
		}
	}
	return cp, nil
}

func removeStaleCheckpoints(parent string) {
	old, _ := filepath.Glob(filepath.Join(parent, "checkpoint-*"))
	for _, dir := range old {
		if fi, err := os.Stat(dir); err == nil && time.Since(fi.ModTime()) > staleCheckpointAge {
			os.RemoveAll(dir)
		}
	}
}

// Rollback restores the checkpoint over the live state.
func (c *Checkpoint) Rollback(backupDir string) error {
	saved := filepath.Join(c.dir, "config.json")
	if _, err := os.Stat(saved); err == nil {
		if err := fsutil.CopyFile(saved, configPath(), 0o600); err != nil {
			return err
		}
	}
	return RestoreState(filepath.Join(c.dir, "state"), backupDir)
}

// Discard deletes the checkpoint.
func (c *Checkpoint) Discard() { os.RemoveAll(c.dir) }

// appName is the macOS app to control (CSWITCH_DESKTOP_APP overrides it for testing).
func appName() string {
	if a := os.Getenv("CSWITCH_DESKTOP_APP"); a != "" {
		return a
	}
	return "Claude"
}

// Supported reports whether Claude Desktop exists on this platform. There is
// no official Claude Desktop for Linux; there cswitch handles Claude Code
// only, and the Desktop data directory simply never exists.
func Supported() bool { return runtime.GOOS == "darwin" }

// Running reports whether the Desktop app is running.
func Running() bool {
	if !Supported() {
		return false
	}
	return exec.Command("/usr/bin/pgrep", "-x", appName()).Run() == nil
}

// Quit asks Desktop to quit and waits for it to exit.
func Quit(timeout time.Duration) error {
	if !Running() {
		return nil
	}
	if out, err := exec.Command("/usr/bin/osascript", "-e", `quit app "`+appName()+`"`).CombinedOutput(); err != nil {
		return i18n.E("err.desktop_quit", err, strings.TrimSpace(string(out)))
	}
	deadline := time.Now().Add(timeout)
	for Running() {
		if time.Now().After(deadline) {
			return i18n.E("err.desktop_quit_timeout", timeout)
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Give it a moment to release file handles after the process is gone.
	time.Sleep(500 * time.Millisecond)
	return nil
}

// Launch opens the Desktop app.
func Launch() error {
	if !Supported() {
		return nil
	}
	if out, err := exec.Command("/usr/bin/open", "-a", appName()).CombinedOutput(); err != nil {
		return i18n.E("err.desktop_open", err, strings.TrimSpace(string(out)))
	}
	return nil
}
