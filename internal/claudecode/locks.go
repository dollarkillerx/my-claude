package claudecode

import (
	"math/rand/v2"
	"os"
	"time"

	"github.com/dollarkillerx/my-claude/internal/i18n"
)

// Claude Code guards its OAuth token refresh and its writes to the global
// config with advisory locks from the npm "proper-lockfile" package: a lock is
// a directory (mkdir is the mutex) whose mtime the holder touches every 5s.
//
//   - A token refresh takes two locks, in order: <config dir>/.oauth_refresh.lock
//     and the legacy <config dir>.lock (~/.claude.lock). They count as stale
//     only after 60s.
//   - A config write locks <config file>.lock (~/.claude.json.lock), stale
//     after 10s.
//
// Holding the credential locks while swapping credentials keeps a swap from
// interleaving with a running claude's refresh, which would otherwise
// overwrite the swapped login with the old account's refreshed token. Under
// the lock, Claude Code's re-read after acquiring sees the new, unexpired
// credentials and skips its refresh. (Protocol as documented by claude-swap,
// verified against the Claude Code 2.1.2xx bundle.)
const (
	credentialLockStale = 60 * time.Second
	configLockStale     = 10 * time.Second
	lockTouchEvery      = 3 * time.Second // a little faster than Claude Code's 5s
)

// lockWait bounds the wait for each lock; Claude Code holds them briefly.
var lockWait = 9 * time.Second

func (h Home) credentialLockPaths() []string {
	dir := h.ConfigDir()
	return []string{dir + "/.oauth_refresh.lock", dir + ".lock"}
}

func (h Home) configLockPath() string { return h.ConfigPath() + ".lock" }

// WithCredentialsLock runs fn holding Claude Code's credential locks.
func (h Home) WithCredentialsLock(fn func() error) error {
	paths := h.credentialLockPaths()
	var releases []func()
	defer func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}()
	for _, p := range paths {
		release, err := acquireDirLock(p, credentialLockStale)
		if err != nil {
			return err
		}
		releases = append(releases, release)
	}
	return fn()
}

// WithConfigLock runs fn holding Claude Code's global config lock.
func (h Home) WithConfigLock(fn func() error) error {
	release, err := acquireDirLock(h.configLockPath(), configLockStale)
	if err != nil {
		return err
	}
	defer release()
	return fn()
}

// acquireDirLock takes a proper-lockfile style lock at path, waiting up to
// lockWait for a live holder and breaking a lock older than stale. The lock
// is touched while held; the returned function releases it.
func acquireDirLock(path string, stale time.Duration) (func(), error) {
	deadline := time.Now().Add(lockWait)
	for {
		err := os.Mkdir(path, 0o755)
		if err == nil {
			break
		}
		if os.IsNotExist(err) {
			// The parent (e.g. a fresh config dir) does not exist yet: no
			// Claude Code can be holding a lock in it.
			return func() {}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if fi, serr := os.Stat(path); serr == nil && time.Since(fi.ModTime()) > stale {
			os.Remove(path) // abandoned by a crashed holder
			continue
		}
		if time.Now().After(deadline) {
			return nil, i18n.E("err.claude_lock_busy", path)
		}
		time.Sleep(time.Duration(300+rand.IntN(400)) * time.Millisecond)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(lockTouchEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-t.C:
				os.Chtimes(path, now, now)
			}
		}
	}()
	return func() {
		close(stop)
		<-done
		os.Remove(path)
	}, nil
}
