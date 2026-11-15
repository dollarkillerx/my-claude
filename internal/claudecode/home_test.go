package claudecode

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHomeLocations(t *testing.T) {
	t.Setenv("CSWITCH_CODE_KEYCHAIN_SERVICE", "")
	dir := "/Users/x/.config/cswitch/sessions/work"
	h := Home{Dir: dir}
	sum := sha256.Sum256([]byte(dir))
	if got, want := h.keychainService(), "Claude Code-credentials-"+hex.EncodeToString(sum[:])[:8]; got != want {
		t.Errorf("keychain service = %q, want %q", got, want)
	}
	if Default.keychainService() != "Claude Code-credentials" {
		t.Errorf("default service = %q", Default.keychainService())
	}
	if h.ConfigPath() != dir+"/.claude.json" || h.ConfigDir() != dir {
		t.Errorf("paths = %q %q", h.ConfigPath(), h.ConfigDir())
	}
}

func TestHomeCredentialsFileBackend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_CODE_CREDENTIALS_FILE", filepath.Join(dir, "default.json"))
	h := Home{Dir: filepath.Join(dir, "session")}
	c, _ := ParseCredentials(`{"claudeAiOauth":{"accessToken":"a","refreshToken":"r","expiresAt":1}}`)
	if err := h.WriteCredentials(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "session", ".credentials.json")); err != nil {
		t.Errorf("session credentials not in the session dir: %v", err)
	}
	if got, err := h.ReadCredentials(); err != nil || got.OAuth.RefreshToken != "r" {
		t.Errorf("read back = %+v, %v", got, err)
	}
	if _, err := Default.ReadCredentials(); err != ErrNotLoggedIn {
		t.Errorf("the default home must be untouched: %v", err)
	}
	if err := h.DeleteCredentials(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ReadCredentials(); err != ErrNotLoggedIn {
		t.Errorf("after delete: %v", err)
	}
}

func TestCredentialLocksFollowClaudeCodesLayout(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CSWITCH_CLAUDE_HOME", filepath.Join(base, ".claude"))
	os.MkdirAll(filepath.Join(base, ".claude"), 0o700)
	err := Default.WithCredentialsLock(func() error {
		for _, p := range []string{filepath.Join(base, ".claude", ".oauth_refresh.lock"), filepath.Join(base, ".claude.lock")} {
			if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
				t.Errorf("lock %s not held as a directory: %v", p, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(base, ".claude", ".oauth_refresh.lock"), filepath.Join(base, ".claude.lock")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("lock %s not released", p)
		}
	}
}

func TestLockWaitsForHolderAndBreaksStale(t *testing.T) {
	old := lockWait
	lockWait = 500 * time.Millisecond
	defer func() { lockWait = old }()
	dir := t.TempDir()
	lock := filepath.Join(dir, "x.lock")

	// A live holder (fresh mtime) is waited for, then reported as busy.
	os.Mkdir(lock, 0o755)
	if _, err := acquireDirLock(lock, time.Minute); err == nil {
		t.Fatal("took a lock that a live holder has")
	}
	// An abandoned one (older than stale) is broken.
	past := time.Now().Add(-2 * time.Minute)
	os.Chtimes(lock, past, past)
	release, err := acquireDirLock(lock, time.Minute)
	if err != nil {
		t.Fatalf("stale lock not broken: %v", err)
	}
	release()
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("not released")
	}
	// A missing parent means nobody can hold it.
	if r, err := acquireDirLock(filepath.Join(dir, "nope", "y.lock"), time.Minute); err != nil {
		t.Errorf("missing parent: %v", err)
	} else {
		r()
	}
}
