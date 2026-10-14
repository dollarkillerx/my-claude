package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const (
	acctA = "aaaaaaaa-0000-0000-0000-000000000001"
	orgA  = "aaaaaaaa-0000-0000-0000-0000000000a1"
	acctB = "bbbbbbbb-0000-0000-0000-000000000002"
	orgB  = "bbbbbbbb-0000-0000-0000-0000000000b2"
)

func TestSyncMergesNewestAndPropagatesDeletes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	root := filepath.Join(dir, "claude-code-sessions")
	a := filepath.Join(root, acctA, orgA)
	b := filepath.Join(root, acctB, orgB)
	old, recent := time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)

	writeFile(t, filepath.Join(a, "local_1.json"), "a1", old)
	writeFile(t, filepath.Join(a, "local_2.json"), "a2-old", old)
	writeFile(t, filepath.Join(b, "local_2.json"), "b2-new", recent)
	writeFile(t, filepath.Join(b, "local_3", "agent", "x.json"), "b3", old)
	writeFile(t, filepath.Join(a, "cowork-clientdata-cache.json"), "account-specific", old)
	writeFile(t, filepath.Join(root, "skills-plugin", orgA, "local_9.json"), "ignored", old)

	state := filepath.Join(t.TempDir(), "state.json")
	trash := filepath.Join(t.TempDir(), "trash")
	res, err := Sync(state, trash)
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 3 {
		t.Errorf("copied = %d, want 3", res.Copied)
	}
	if got := readFile(t, filepath.Join(b, "local_1.json")); got != "a1" {
		t.Errorf("b/local_1 = %q", got)
	}
	if got := readFile(t, filepath.Join(a, "local_2.json")); got != "b2-new" {
		t.Errorf("a/local_2 = %q, want newest copy", got)
	}
	if got := readFile(t, filepath.Join(a, "local_3", "agent", "x.json")); got != "b3" {
		t.Errorf("a/local_3/agent/x.json = %q", got)
	}
	if _, err := os.Stat(filepath.Join(b, "cowork-clientdata-cache.json")); !os.IsNotExist(err) {
		t.Errorf("non-session file was copied")
	}

	// Second run is a no-op.
	if res, err := Sync(state, trash); err != nil || res.Copied != 0 || res.Removed != 0 {
		t.Fatalf("second sync = %+v, %v", res, err)
	}

	// Deleting a session in one account removes it everywhere instead of restoring it.
	os.Remove(filepath.Join(b, "local_1.json"))
	res, err = Sync(state, trash)
	if err != nil {
		t.Fatal(err)
	}
	if res.Removed != 1 || res.Copied != 0 {
		t.Errorf("after delete: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(a, "local_1.json")); !os.IsNotExist(err) {
		t.Errorf("deleted session came back in account A")
	}
	matches, _ := filepath.Glob(filepath.Join(trash, "*", "claude-code-sessions", acctA, orgA, "local_1.json"))
	if len(matches) != 1 {
		t.Errorf("deleted session not in trash: %v", matches)
	}

	// A new account folder receives everything.
	c := filepath.Join(root, "cccccccc-0000-0000-0000-000000000003", "cccccccc-0000-0000-0000-0000000000c3")
	os.MkdirAll(c, 0o755)
	if res, err := Sync(state, trash); err != nil || res.Copied != 2 {
		t.Fatalf("new scope sync = %+v, %v", res, err)
	}
}

func TestSnapshotRestoreKeepsOtherKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	cfg := filepath.Join(dir, "config.json")
	writeFile(t, cfg, `{"locale":"zh-CN","oauth:tokenCache":"encA","oauth:tokenCacheV2":"encA2","lastKnownAccountUuid":"`+acctA+`"}`, time.Now())

	snapA, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if snapA.AccountUUID != acctA || len(snapA.Keys) != 3 {
		t.Fatalf("snapshot = %+v", snapA)
	}

	backups := t.TempDir()
	if err := Restore(nil, backups); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(); err != ErrNotLoggedIn {
		t.Fatalf("after clear: %v", err)
	}

	snapB := &Snapshot{AccountUUID: acctB, Keys: map[string]json.RawMessage{
		"oauth:tokenCacheV2":   json.RawMessage(`"encB"`),
		"lastKnownAccountUuid": json.RawMessage(`"` + acctB + `"`),
	}}
	if err := Restore(snapB, backups); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal([]byte(readFile(t, cfg)), &m)
	if m["locale"] != "zh-CN" || m["oauth:tokenCacheV2"] != "encB" || m["lastKnownAccountUuid"] != acctB {
		t.Errorf("config after restore = %v", m)
	}
	if _, ok := m["oauth:tokenCache"]; ok {
		t.Errorf("stale oauth:tokenCache from account A survived")
	}
	if CurrentAccount() != acctB {
		t.Errorf("CurrentAccount = %q", CurrentAccount())
	}
}

func TestStateRoundTripDropsStaleJournal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	now := time.Now()
	writeFile(t, filepath.Join(dir, "Cookies"), "cookiesA", now)
	writeFile(t, filepath.Join(dir, "Local Storage", "leveldb", "000003.log"), "storageA", now)

	saved := t.TempDir()
	if err := SaveState(saved); err != nil {
		t.Fatal(err)
	}
	// Account B's state, including a journal and storage A never had.
	writeFile(t, filepath.Join(dir, "Cookies"), "cookiesB", now)
	writeFile(t, filepath.Join(dir, "Cookies-journal"), "journalB", now)
	writeFile(t, filepath.Join(dir, "Local Storage", "leveldb", "000004.log"), "storageB", now)
	writeFile(t, filepath.Join(dir, "IndexedDB", "x"), "idbB", now)

	backups := t.TempDir()
	if err := RestoreState(saved, backups); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "Cookies")); got != "cookiesA" {
		t.Errorf("Cookies = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Local Storage", "leveldb", "000003.log")); got != "storageA" {
		t.Errorf("Local Storage = %q", got)
	}
	for _, stale := range []string{"Cookies-journal", "IndexedDB", filepath.Join("Local Storage", "leveldb", "000004.log")} {
		if _, err := os.Stat(filepath.Join(dir, stale)); !os.IsNotExist(err) {
			t.Errorf("%s from account B left behind", stale)
		}
	}
	if b, _ := filepath.Glob(filepath.Join(backups, "IndexedDB.*")); len(b) != 1 {
		t.Errorf("B's IndexedDB not backed up: %v", b)
	}
}

func TestCheckpointRollback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	now := time.Now()
	writeFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCacheV2":"encA","lastKnownAccountUuid":"`+acctA+`"}`, now)
	writeFile(t, filepath.Join(dir, "Cookies"), "cookiesA", now)

	cp, err := NewCheckpoint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Discard()

	// A half-done switch: tokens replaced, cookies gone.
	snapB := &Snapshot{AccountUUID: acctB, Keys: map[string]json.RawMessage{
		"oauth:tokenCacheV2":   json.RawMessage(`"encB"`),
		"lastKnownAccountUuid": json.RawMessage(`"` + acctB + `"`),
	}}
	backups := t.TempDir()
	if err := Restore(snapB, backups); err != nil {
		t.Fatal(err)
	}
	if err := RestoreState("", backups); err != nil {
		t.Fatal(err)
	}

	if err := cp.Rollback(backups); err != nil {
		t.Fatal(err)
	}
	if CurrentAccount() != acctA {
		t.Errorf("account after rollback = %q", CurrentAccount())
	}
	if got := readFile(t, filepath.Join(dir, "Cookies")); got != "cookiesA" {
		t.Errorf("Cookies after rollback = %q", got)
	}
}

func TestReadTokensWithoutAccount(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	writeFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCacheV2":"enc"}`, time.Now())
	s, err := Read()
	if err != nil {
		t.Fatalf("tokens without an account must not read as logged out: %v", err)
	}
	if s.AccountUUID != "" {
		t.Errorf("AccountUUID = %q", s.AccountUUID)
	}
	writeFile(t, filepath.Join(dir, "config.json"), `{"locale":"en"}`, time.Now())
	if _, err := Read(); err != ErrNotLoggedIn {
		t.Errorf("no tokens: %v", err)
	}
}

func TestBlankTokensAreLoggedOut(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	for _, blank := range []string{`null`, `""`, `{}`, `[]`} {
		writeFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCache":`+blank+`,"oauth:tokenCacheV2":""," lastKnownAccountUuid":"x","lastKnownAccountUuid":"`+acctA+`"}`, time.Now())
		if _, err := Read(); err != ErrNotLoggedIn {
			t.Errorf("blank token %s: err = %v, want ErrNotLoggedIn", blank, err)
		}
	}
	writeFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCache":"","oauth:tokenCacheV2":"enc","lastKnownAccountUuid":"`+acctA+`"}`, time.Now())
	if s, err := Read(); err != nil || s.AccountUUID != acctA {
		t.Errorf("one real token: %v, %v", s, err)
	}
}

func TestStaleCheckpointsRemoved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	parent := t.TempDir()
	stale := filepath.Join(parent, "checkpoint-old")
	fresh := filepath.Join(parent, "checkpoint-running")
	writeFile(t, filepath.Join(stale, "state", "Cookies"), "secret", time.Now())
	writeFile(t, filepath.Join(fresh, "state", "Cookies"), "secret", time.Now())
	old := time.Now().Add(-time.Hour)
	os.Chtimes(stale, old, old)

	cp, err := NewCheckpoint(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Discard()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale checkpoint not removed")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("a checkpoint of a run in progress was removed")
	}
}

func TestSyncNewerEditBeatsDeletion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	root := filepath.Join(dir, "claude-code-sessions")
	a := filepath.Join(root, acctA, orgA)
	b := filepath.Join(root, acctB, orgB)
	old := time.Now().Add(-time.Hour)
	writeFile(t, filepath.Join(a, "local_x.json"), "v1", old)

	state := filepath.Join(t.TempDir(), "state.json")
	trash := t.TempDir()
	if _, err := Sync(state, trash); err != nil {
		t.Fatal(err)
	}

	// Deleted in A, but edited in B after the last sync.
	os.Remove(filepath.Join(a, "local_x.json"))
	writeFile(t, filepath.Join(b, "local_x.json"), "v2-edited", time.Now().Add(time.Second))

	res, err := Sync(state, trash)
	if err != nil {
		t.Fatal(err)
	}
	if res.Removed != 0 {
		t.Errorf("edited session was trashed: %+v", res)
	}
	if got := readFile(t, filepath.Join(a, "local_x.json")); got != "v2-edited" {
		t.Errorf("A should get the edited copy back, got %q", got)
	}
}

func TestSyncIgnoresOldStateFormat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSWITCH_DESKTOP_DIR", dir)
	a := filepath.Join(dir, "claude-code-sessions", acctA, orgA)
	writeFile(t, filepath.Join(a, "local_1.json"), "x", time.Now())
	state := filepath.Join(t.TempDir(), "state.json")
	// Format written by earlier versions: a bare map of file lists.
	os.WriteFile(state, []byte(`{"`+a+`":["local_1.json","local_gone.json"]}`), 0o600)
	if res, err := Sync(state, t.TempDir()); err != nil || res.Removed != 0 {
		t.Errorf("old state: %+v, %v", res, err)
	}
}
