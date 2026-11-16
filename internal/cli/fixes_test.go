package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/store"
)

// sessionCreds writes credentials for token "dummy-<who>" into a session home.
func sessionCreds(t *testing.T, home claudecode.Home, who string, expires time.Time) {
	t.Helper()
	c, err := claudecode.ParseCredentials(fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"dummy-%s","refreshToken":"dummy-r-%s","expiresAt":%d,"scopes":[],"subscriptionType":"max"}}`, who, who, expires.UnixMilli()))
	if err != nil {
		t.Fatal(err)
	}
	if err := home.WriteCredentials(c); err != nil {
		t.Fatal(err)
	}
}

func savedToken(t *testing.T, name string) string {
	t.Helper()
	raw, err := store.Secret(name, "code")
	if err != nil {
		t.Fatal(err)
	}
	c, err := claudecode.ParseCredentials(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c.OAuth.AccessToken
}

func startedSession(t *testing.T, home claudecode.Home) (stop func()) {
	t.Helper()
	pidFile := filepath.Join(pidDir(home), fmt.Sprint(os.Getpid()))
	os.MkdirAll(pidDir(home), 0o700)
	os.WriteFile(pidFile, nil, 0o600)
	return func() { os.Remove(pidFile) }
}

// A session that refreshed its token and then ended without cswitch seeing
// it must not have that token overwritten by the profile's stale one, and
// usage must take it over rather than drop the login as ended.
func TestSessionLoginSurvivesUncleanExit(t *testing.T) {
	_, _, api := twoAccounts(t, "beta")
	api.owners["alpha2"] = api.owners["alpha"]
	p, _ := store.Load("alpha")
	home := sessionHome(p)
	if err := prepareSession(p, home); err != nil {
		t.Fatal(err)
	}
	sessionCreds(t, home, "alpha2", time.Now().Add(2*time.Hour)) // the session refreshed

	if err := prepareSession(p, home); err != nil {
		t.Fatal(err)
	}
	c, _ := home.ReadCredentials()
	if c.OAuth.AccessToken != "dummy-alpha2" {
		t.Errorf("session token overwritten with %s", c.OAuth.AccessToken)
	}
	if savedToken(t, "alpha") != "dummy-alpha2" {
		t.Error("newer session token not saved into the profile")
	}

	// A profile whose login was dropped gets it back from the session.
	sessionCreds(t, home, "alpha2", time.Now().Add(3*time.Hour))
	store.DeleteSecret("alpha", "code")
	fresh, _ := store.Load("alpha")
	fresh.HasCode = false
	store.Save(fresh)
	fresh, _ = store.Load("alpha")
	if rd := readUsage(t.Context(), fresh, false, true); rd.err != nil || rd.report == nil {
		t.Fatalf("usage after adopting: %+v", rd)
	}
	if fresh, _ = store.Load("alpha"); !fresh.HasCode {
		t.Error("login not restored")
	}

	// A login saved after the session ended is newer and is what the next
	// session starts with.
	api.owners["alpha3"] = api.owners["alpha"]
	cf := filepath.Join(t.TempDir(), "c.json")
	writeCreds(t, cf, "alpha3", time.Now().Add(5*time.Hour))
	data, _ := os.ReadFile(cf)
	store.SetSecret("alpha", "code", string(data))
	if err := prepareSession(fresh, home); err != nil {
		t.Fatal(err)
	}
	if c, _ := home.ReadCredentials(); c.OAuth.AccessToken != "dummy-alpha3" {
		t.Errorf("session not given the newer profile login: %s", c.OAuth.AccessToken)
	}
}

func TestRunSessionWithClaude(t *testing.T) {
	_, _, _ = twoAccounts(t, "beta")
	bin := t.TempDir()
	// A claude that refreshes its token and records its config dir.
	script := `#!/bin/sh
printf '{"claudeAiOauth":{"accessToken":"dummy-alpha9","refreshToken":"dummy-r-alpha9","expiresAt":%s000,"scopes":[]}}' "$(($(date +%s)+7200))" > "$CLAUDE_CONFIG_DIR/.credentials.json"
echo "$CLAUDE_CONFIG_DIR $*" > "` + bin + `/out"
ls "$CLAUDE_CONFIG_DIR/.cswitch-pids" >> "` + bin + `/out"
`
	os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	p, _ := store.Load("alpha")
	if err := runSession(p, []string{"--resume"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(bin, "out"))
	if !strings.Contains(string(out), sessionHome(p).Dir+" --resume") || !strings.Contains(string(out), fmt.Sprint(os.Getpid())) {
		t.Errorf("claude saw %q", out)
	}
	if savedToken(t, "alpha") != "dummy-alpha9" {
		t.Error("refreshed token not saved back")
	}
	if sessionRunning(p) {
		t.Error("pid file left behind")
	}
}

func TestRemoveAndRenameDropSession(t *testing.T) {
	_, run, _ := twoAccounts(t, "beta")
	p, _ := store.Load("alpha")
	home := sessionHome(p)
	if err := prepareSession(p, home); err != nil {
		t.Fatal(err)
	}
	sessionCreds(t, home, "alpha2", time.Now().Add(2*time.Hour))

	stop := startedSession(t, home)
	if err := run("rename", "alpha", "first"); err == nil || !strings.Contains(err.Error(), "session") {
		t.Errorf("renamed under a running session: %v", err)
	}
	stop()

	if err := run("rename", "alpha", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home.Dir); !os.IsNotExist(err) {
		t.Error("old session directory left behind")
	}
	if savedToken(t, "first") != "dummy-alpha2" {
		t.Error("the session's newer token was lost in the rename")
	}

	first, _ := store.Load("first")
	if err := prepareSession(first, sessionHome(first)); err != nil {
		t.Fatal(err)
	}
	shared := os.Getenv("CSWITCH_CLAUDE_HOME")
	os.MkdirAll(filepath.Join(shared, "projects"), 0o700)
	os.WriteFile(filepath.Join(shared, "projects", "keep"), nil, 0o600)
	os.Symlink(filepath.Join(shared, "projects"), filepath.Join(sessionHome(first).Dir, "projects"))
	if err := run("remove", "first", "-y"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionHome(first).Dir); !os.IsNotExist(err) {
		t.Error("session directory left behind")
	}
	if _, err := os.Stat(filepath.Join(shared, "projects", "keep")); err != nil {
		t.Error("removing the session deleted shared files")
	}
}

func TestClaimUsesSessionToken(t *testing.T) {
	_, run, api := twoAccounts(t, "beta")
	api.owners["alpha2"] = api.owners["alpha"]
	api.usage["alpha2"] = usageBody(100, 100, cedar)
	p, _ := store.Load("alpha")
	home := sessionHome(p)
	prepareSession(p, home)
	sessionCreds(t, home, "alpha2", time.Now().Add(time.Hour))
	defer startedSession(t, home)()

	if err := run("reset", "alpha", "-y"); err != nil {
		t.Fatal(err)
	}
	if len(api.claimTokens) != 1 || api.claimTokens[0] != "dummy-alpha2" {
		t.Errorf("claimed with %v, want the session's token", api.claimTokens)
	}
}

func TestResetRefusedLocally(t *testing.T) {
	_, run, api := twoAccounts(t, "")
	for name, body := range map[string]string{
		"unavailable": strings.Replace(strings.Replace(cedar, `"usable_now":true`, `"usable_now":false,"unavailable_reason":"region"`, 1), `"use_requires_limit":true`, `"use_requires_limit":false`, 1),
		"cooldown":    strings.Replace(cedar, `"next_grant_id"`, `"cooldown_until":"2099-01-01T00:00:00Z","next_grant_id"`, 1),
	} {
		api.usage["alpha"] = usageBody(100, 100, body)
		forgetUsage(&store.Profile{AccountUUID: uuidAlpha})
		err := run("reset", "alpha", "-y")
		if err == nil || !strings.Contains(err.Error(), map[string]string{"unavailable": "region", "cooldown": "cooling"}[name]) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(api.claims) != 0 {
		t.Errorf("claims sent: %v", api.claims)
	}
}

func TestAutoIgnoresStaleReadings(t *testing.T) {
	_, _, api := twoAccounts(t, "alpha")
	api.usage["alpha"] = usageBody(95, 40, "")
	api.usage["beta"] = usageBody(10, 20, "")
	beta, _ := store.Load("beta")
	readUsage(t.Context(), beta, false, false)

	// beta's only reading is old and its endpoint is backing off.
	cacheMu.Lock()
	e := cacheLocked()[cacheKey(beta)]
	e.FetchedAt = time.Now().Add(-time.Hour)
	e.BackoffUntil = time.Now().Add(time.Hour)
	cacheMu.Unlock()

	if err := autoTick(t.Context(), autoOptions{Threshold: 90, WeeklyThreshold: 90}); err != nil {
		t.Fatal(err)
	}
	if liveTokenOf(t) != "dummy-alpha" {
		t.Error("switched on an hour-old reading")
	}

	// The same holds for the account in use.
	alpha, _ := store.Load("alpha")
	cacheMu.Lock()
	e = cacheLocked()[cacheKey(alpha)]
	e.FetchedAt = time.Now().Add(-time.Hour)
	e.BackoffUntil = time.Now().Add(time.Hour)
	cacheMu.Unlock()
	if err := autoTick(t.Context(), autoOptions{Threshold: 90, WeeklyThreshold: 90}); err == nil || !strings.Contains(err.Error(), "alpha") {
		t.Errorf("stale reading of the account in use: %v", err)
	}
}

// Another cswitch process writing the cache meanwhile keeps its entries and
// its backoff.
func TestUsageCacheMergesProcesses(t *testing.T) {
	_, _, api := twoAccounts(t, "")
	alpha, _ := store.Load("alpha")
	beta, _ := store.Load("beta")
	readUsage(t.Context(), alpha, false, false)
	saveUsageCache()

	// Another process backs beta off and records an account of its own.
	other := fmt.Sprintf(`{%q:{"fetchedAt":%q,"backoffUntil":%q,"failures":1},"someone":{"fetchedAt":%q}}`,
		cacheKey(beta), time.Now().Format(time.RFC3339Nano), time.Now().Add(time.Hour).Format(time.RFC3339Nano), time.Now().Format(time.RFC3339Nano))
	data, _ := os.ReadFile(usageCachePath())
	merged := strings.TrimSuffix(string(data), "}") + "," + strings.TrimPrefix(other, "{")
	os.WriteFile(usageCachePath(), []byte(merged), 0o600)
	future := time.Now().Add(time.Second)
	os.Chtimes(usageCachePath(), future, future)

	if rd := readUsage(t.Context(), beta, false, false); rd.err == nil || api.usageHits["beta"] != 0 {
		t.Errorf("other process's backoff ignored: %+v, %d hits", rd, api.usageHits["beta"])
	}
	readUsage(t.Context(), alpha, false, true)
	saveUsageCache()
	data, _ = os.ReadFile(usageCachePath())
	for _, k := range []string{cacheKey(alpha), cacheKey(beta), "someone", "backoffUntil"} {
		if !strings.Contains(string(data), k) {
			t.Errorf("cache lost %q: %s", k, data)
		}
	}
}

func TestRotateAutoLog(t *testing.T) {
	t.Setenv("CSWITCH_HOME", t.TempDir())
	old := autoLogMax
	autoLogMax = 100
	defer func() { autoLogMax = old }()
	f, err := os.OpenFile(autoLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	stdout := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = stdout }()

	fmt.Fprint(f, strings.Repeat("x", 150))
	rotateAutoLog()
	fmt.Fprint(f, "next")
	if data, _ := os.ReadFile(autoLogPath()); string(data) != "next" {
		t.Errorf("log after rotation = %q", data)
	}
	if data, _ := os.ReadFile(autoLogPath() + ".1"); len(data) != 150 {
		t.Errorf("rotated log has %d bytes", len(data))
	}
	os.Stdout = stdout // not the log: left alone
	fmt.Fprint(f, strings.Repeat("y", 150))
	rotateAutoLog()
	if fi, _ := os.Stat(autoLogPath()); fi.Size() < 150 {
		t.Error("rotated a log that is not this process's output")
	}
}

func TestParseRunArgs(t *testing.T) {
	for _, c := range []struct {
		in   []string
		name string
		rest []string
		help bool
	}{
		{nil, "", nil, false},
		{[]string{"work"}, "work", nil, false},
		{[]string{"work", "--resume"}, "work", []string{"--resume"}, false},
		{[]string{"work", "--", "--resume"}, "work", []string{"--resume"}, false},
		{[]string{"work", "-p", "hi", "--", "x"}, "work", []string{"-p", "hi", "--", "x"}, false},
		{[]string{"--resume"}, "", []string{"--resume"}, false},
		{[]string{"--", "fix it"}, "", []string{"fix it"}, false},
		{[]string{"--lang", "ja", "work", "-c"}, "work", []string{"-c"}, false},
		{[]string{"--lang=zh", "--help"}, "", nil, true},
		{[]string{"work", "--help"}, "work", []string{"--help"}, false},
	} {
		name, rest, help := parseRunArgs(c.in)
		if name != c.name || !reflect.DeepEqual(rest, c.rest) && len(rest)+len(c.rest) > 0 || help != c.help {
			t.Errorf("parseRunArgs(%q) = %q %q %v", c.in, name, rest, help)
		}
	}
}

// A server that says Claude Code is too old gets `claude update` once, then
// a fresh reading; --no-update leaves Claude Code alone.
func TestResetsUpdateClaudeCode(t *testing.T) {
	_, run, api := twoAccounts(t, "")
	bin := t.TempDir()
	log := filepath.Join(bin, "out")
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"$*\" >> "+log+"\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := `,"cedar_ember":{"eligible":false,"ineligible_reason":"cli_version"}`
	api.usage["alpha"] = usageBody(100, 100, old)
	api.usage["beta"] = usageBody(10, 10, old)

	if err := run("resets"); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(log)
	if string(out) != "update\n" {
		t.Errorf("claude ran with %q, want one update", out)
	}
	if api.usageHits["alpha"] != 2 || api.usageHits["beta"] != 2 {
		t.Errorf("status not read again after the update: %v", api.usageHits)
	}

	err := run("reset", "alpha", "-y")
	if err == nil || !strings.Contains(err.Error(), "cli_version") {
		t.Errorf("reset: %v", err)
	}
	if out, _ = os.ReadFile(log); string(out) != "update\nupdate\n" {
		t.Errorf("reset did not update Claude Code: %q", out)
	}

	os.Remove(log)
	run("resets", "--no-update")
	run("reset", "alpha", "-y", "--no-update")
	if _, err := os.Stat(log); err == nil {
		t.Error("claude update ran under --no-update")
	}
	if len(api.claims) != 0 {
		t.Errorf("claims sent: %v", api.claims)
	}
}
