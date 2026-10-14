package desktop

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
)

// Desktop keeps local sessions per account and organization:
//
//	<root>/<accountUuid>/<orgUuid>/local_<id>.json   (plus local_<id>/ dirs)
//
// The transcripts themselves live in ~/.claude/projects and are already shared,
// and the session files do not embed the account or org IDs, so copying them
// between account directories makes every account see every local session.
var sessionRoots = []string{"claude-code-sessions", "local-agent-mode-sessions"}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type fileVersion struct {
	path  string
	mtime time.Time
}

// SyncResult reports what Sync did.
type SyncResult struct {
	Scopes  int // account/org directories taking part
	Copied  int // files written
	Removed int // files moved to trash because they were deleted elsewhere
}

// syncState records, per scope directory, the files present after the last
// sync, and when that sync ran. A file that was there last time and is gone
// now was deleted by the user, so the deletion is propagated instead of the
// file being copied back, unless another scope edited it since the last sync,
// in which case the edit wins.
type syncState struct {
	Time  time.Time           `json:"time"`
	Files map[string][]string `json:"files"`
}

// Sync merges local sessions across all account/org directories: for every
// relative path under a local_* entry, the newest copy is written to every
// directory that lacks it or holds an older one. Desktop must not be running.
// statePath stores the post-sync file lists; propagated deletions are moved
// into trashDir rather than deleted.
func Sync(statePath, trashDir string) (SyncResult, error) {
	var res SyncResult
	var prev syncState
	if data, err := os.ReadFile(statePath); err == nil {
		// An unreadable or old-format state just means no deletions are detected.
		_ = json.Unmarshal(data, &prev)
	}
	next := syncState{Time: time.Now(), Files: map[string][]string{}}
	for _, root := range sessionRoots {
		if err := syncRoot(filepath.Join(Dir(), root), prev, next, trashDir, &res); err != nil {
			return res, err
		}
	}
	data, err := json.Marshal(next)
	if err != nil {
		return res, err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		return res, err
	}
	return res, fsutil.AtomicWrite(statePath, data, 0o600)
}

func syncRoot(root string, prev, next syncState, trashDir string, res *SyncResult) error {
	scopes, err := scopeDirs(root)
	if err != nil {
		return err
	}
	res.Scopes += len(scopes)

	present := map[string]map[string]fileVersion{} // scope -> rel -> version
	for _, dir := range scopes {
		files := map[string]fileVersion{}
		err := walkSessions(dir, func(rel, path string, mtime time.Time) {
			files[rel] = fileVersion{path, mtime}
		})
		if err != nil {
			return err
		}
		present[dir] = files
	}

	// Paths that vanished from a scope since the last sync were deleted there,
	// unless some scope still holds a copy modified after the last sync.
	deleted := map[string]bool{}
	for _, dir := range scopes {
		for _, rel := range prev.Files[dir] {
			if _, ok := present[dir][rel]; !ok {
				deleted[rel] = true
			}
		}
	}
	for rel := range deleted {
		for _, dir := range scopes {
			if v, ok := present[dir][rel]; ok && v.mtime.After(prev.Time) {
				delete(deleted, rel)
				break
			}
		}
	}

	newest := map[string]fileVersion{}
	for _, dir := range scopes {
		for rel, v := range present[dir] {
			if deleted[rel] {
				if err := moveToTrash(v.path, root, trashDir); err != nil {
					return err
				}
				delete(present[dir], rel)
				res.Removed++
				continue
			}
			if cur, ok := newest[rel]; !ok || v.mtime.After(cur.mtime) {
				newest[rel] = v
			}
		}
	}

	for _, dir := range scopes {
		for rel, src := range newest {
			if cur, ok := present[dir][rel]; !ok || src.mtime.After(cur.mtime) {
				if err := fsutil.CopyFile(src.path, filepath.Join(dir, rel), 0o644); err != nil {
					return err
				}
				res.Copied++
			}
			next.Files[dir] = append(next.Files[dir], rel)
		}
	}
	return nil
}

func moveToTrash(path, root, trashDir string) error {
	rel, err := filepath.Rel(filepath.Dir(root), path)
	if err != nil {
		return err
	}
	dst := filepath.Join(trashDir, time.Now().Format("20060102"), rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.Rename(path, dst)
}

// scopeDirs lists <root>/<accountUuid>/<orgUuid> directories.
func scopeDirs(root string) ([]string, error) {
	accts, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, a := range accts {
		if !a.IsDir() || !uuidRe.MatchString(a.Name()) {
			continue
		}
		orgs, err := os.ReadDir(filepath.Join(root, a.Name()))
		if err != nil {
			return nil, err
		}
		for _, o := range orgs {
			if o.IsDir() && uuidRe.MatchString(o.Name()) {
				out = append(out, filepath.Join(root, a.Name(), o.Name()))
			}
		}
	}
	return out, nil
}

// walkSessions calls fn for every regular file under the local_* entries of dir.
func walkSessions(dir string, fn func(rel, path string, mtime time.Time)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "local_") {
			continue
		}
		err := filepath.WalkDir(filepath.Join(dir, e.Name()), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			fn(rel, path, fi.ModTime())
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
