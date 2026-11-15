package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

// Usage readings are cached per account so that the dashboard, the picker,
// `usage` and the auto-switcher do not keep hitting the usage endpoint, which
// is itself rate limited:
//
//   - a reading is reused for pollActive (the account in use, or one near a
//     limit, whose numbers matter now) or pollIdle (every other account);
//   - a 429 backs off for the server's Retry-After, or 1, 2, 4 … up to 30
//     minutes, during which the last reading is shown, marked with its age.
const (
	pollActive  = 1 * time.Minute
	pollIdle    = 5 * time.Minute
	nearLimit   = 80.0
	backoffBase = 1 * time.Minute
	backoffMax  = 30 * time.Minute
)

type cacheEntry struct {
	FetchedAt    time.Time       `json:"fetchedAt"`
	Body         json.RawMessage `json:"body,omitempty"`
	BackoffUntil time.Time       `json:"backoffUntil,omitempty"`
	Failures     int             `json:"failures,omitempty"`
}

// The cache is shared with other cswitch processes (the auto-switcher runs
// all the time, next to commands typed in a terminal). Each process keeps
// what it read, reloads the file when another process has written it, and
// writes back only the entries it changed, merged into the file under a lock,
// so one process never undoes another's reading or rate-limit backoff.
var (
	cacheMu      sync.Mutex
	cacheEntries map[string]*cacheEntry
	cacheDirty   map[string]bool // changed (or deleted) here since the last save
	cachePath    string
	cacheMod     time.Time // modification time of the file as last read
)

func usageCachePath() string { return filepath.Join(store.Home(), "usage-cache.json") }

func readCacheFile(path string) (map[string]*cacheEntry, time.Time) {
	entries := map[string]*cacheEntry{}
	fi, err := os.Stat(path)
	if err != nil {
		return entries, time.Time{}
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &entries)
	}
	for k, e := range entries {
		if e == nil {
			delete(entries, k)
		}
	}
	return entries, fi.ModTime()
}

// overlayDirty applies this process's changes to entries read from the file.
func overlayDirty(disk map[string]*cacheEntry) map[string]*cacheEntry {
	for k := range cacheDirty {
		if e, ok := cacheEntries[k]; ok {
			disk[k] = e
		} else {
			delete(disk, k)
		}
	}
	return disk
}

// cacheLocked returns the in-memory cache, (re)loading it when the file
// changed or CSWITCH_HOME did (as in tests). cacheMu must be held.
func cacheLocked() map[string]*cacheEntry {
	p := usageCachePath()
	if cachePath != p {
		cacheEntries, cacheDirty, cachePath, cacheMod = nil, map[string]bool{}, p, time.Time{}
	}
	var mod time.Time
	if fi, err := os.Stat(p); err == nil {
		mod = fi.ModTime()
	}
	if cacheEntries == nil || !mod.Equal(cacheMod) {
		disk, m := readCacheFile(p)
		if cacheEntries != nil {
			disk = overlayDirty(disk)
		}
		cacheEntries, cacheMod = disk, m
	}
	return cacheEntries
}

// setCacheEntry and dropCacheEntry change the cache. cacheMu must be held.
func setCacheEntry(key string, e *cacheEntry) {
	cacheLocked()[key] = e
	cacheDirty[key] = true
}

func dropCacheEntry(key string) {
	delete(cacheLocked(), key)
	cacheDirty[key] = true
}

// saveUsageCache merges this process's changes into the file; failures only
// cost a later refetch.
func saveUsageCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheEntries == nil || len(cacheDirty) == 0 {
		return
	}
	if os.MkdirAll(filepath.Dir(cachePath), 0o700) != nil {
		return
	}
	lock, err := os.OpenFile(cachePath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX) != nil {
		return
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	disk, _ := readCacheFile(cachePath)
	merged := overlayDirty(disk)
	data, err := json.Marshal(merged)
	if err != nil {
		return
	}
	if fsutil.AtomicWrite(cachePath, data, 0o600) != nil {
		return
	}
	cacheEntries, cacheDirty = merged, map[string]bool{}
	if fi, err := os.Stat(cachePath); err == nil {
		cacheMod = fi.ModTime()
	}
}

func cacheKey(p *store.Profile) string {
	if p.AccountUUID != "" {
		return p.AccountUUID
	}
	return "name:" + p.Name
}

// pollInterval is how long a reading stays good enough to reuse.
func pollInterval(r *usage.Report, active bool) time.Duration {
	if active || maxPercent(r) >= nearLimit {
		return pollActive
	}
	return pollIdle
}

// maxPercent is the highest utilization across an account's limits.
func maxPercent(r *usage.Report) float64 {
	if r == nil {
		return 0
	}
	m := 0.0
	for _, l := range r.Limits {
		m = max(m, l.Percent)
	}
	if len(r.Limits) == 0 {
		for _, w := range r.Windows {
			if knownWindow(w.Name) {
				m = max(m, w.Utilization)
			}
		}
	}
	return m
}

// reading is one account's usage as far as it is known: report may be a
// stale one (age says how old) shown alongside err.
type reading struct {
	report *usage.Report
	age    time.Duration
	err    error
}

// readUsage returns p's usage from the cache when it is recent enough for
// the polling policy, and fetches it otherwise. force skips the freshness
// check but never a rate-limit backoff. Call saveUsageCache afterwards.
func readUsage(ctx context.Context, p *store.Profile, live, force bool) reading {
	key := cacheKey(p)
	now := time.Now()

	cacheMu.Lock()
	e := cacheLocked()[key]
	var cached *usage.Report
	var age time.Duration
	if e != nil && len(e.Body) > 0 {
		cached, _ = usage.Parse(e.Body)
		age = now.Sub(e.FetchedAt)
	}
	if e != nil && now.Before(e.BackoffUntil) {
		cacheMu.Unlock()
		return reading{cached, age, i18n.E("err.usage_rate_limited", int(e.BackoffUntil.Sub(now).Minutes())+1)}
	}
	if !force && cached != nil && age < pollInterval(cached, live) {
		cacheMu.Unlock()
		return reading{cached, age, nil}
	}
	cacheMu.Unlock()

	body, err := fetchUsageRaw(ctx, p, live)

	cacheMu.Lock()
	defer cacheMu.Unlock()
	entries := cacheLocked()
	if err == nil {
		r, perr := usage.Parse(body)
		if perr != nil {
			return reading{cached, age, perr}
		}
		setCacheEntry(key, &cacheEntry{FetchedAt: now, Body: body})
		return reading{r, 0, nil}
	}
	if limited, retryAfter := usage.IsRateLimited(err); limited {
		// Start from the entry as it is now: another process may have
		// updated it while this one was fetching.
		e := &cacheEntry{}
		if cur := entries[key]; cur != nil {
			*e = *cur
		}
		e.Failures++
		wait := retryAfter
		if wait <= 0 {
			wait = min(backoffMax, backoffBase<<min(e.Failures-1, 5))
		}
		e.BackoffUntil = now.Add(wait)
		setCacheEntry(key, e)
		return reading{cached, age, i18n.E("err.usage_rate_limited", int(wait.Minutes())+1)}
	}
	return reading{cached, age, err}
}

// profileUsage is readUsage without a stale fallback, for callers that only
// want a usable report.
func profileUsage(ctx context.Context, p *store.Profile, live bool) (*usage.Report, error) {
	rd := readUsage(ctx, p, live, false)
	if rd.err != nil && rd.report == nil {
		return nil, rd.err
	}
	return rd.report, nil
}
