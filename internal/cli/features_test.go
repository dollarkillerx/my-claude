package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

func usageBody(session, weekly float64, extra string) string {
	return fmt.Sprintf(`{"limits":[{"kind":"session","group":"session","percent":%v,"resets_at":"2030-01-01T00:00:00Z","is_active":true},`+
		`{"kind":"weekly_all","group":"weekly","percent":%v,"resets_at":"2030-01-05T00:00:00Z","is_active":true}]%s}`, session, weekly, extra)
}

// twoAccounts saves alpha and beta (both with Claude Code logins) and leaves
// Claude Code logged in as live ("alpha" or "beta", or "" for logged out).
func twoAccounts(t *testing.T, live string) (credFile string, run func(...string) error, api *fakeAPI) {
	t.Helper()
	credFile, run = fileEnv(t)
	api = newFakeAPI(t, map[string][2]string{
		"alpha": {uuidAlpha, "alpha@example.com"},
		"beta":  {uuidBeta, "beta@example.com"},
	})
	for _, who := range []string{"alpha", "beta"} {
		writeCreds(t, credFile, who, time.Now().Add(time.Hour))
		if err := run("add", "--code-only"); err != nil {
			t.Fatal(err)
		}
	}
	switch live {
	case "":
		os.Remove(credFile)
		resetLiveCache()
	case "alpha":
		if err := run("use", "alpha", "--code-only"); err != nil {
			t.Fatal(err)
		}
	}
	return credFile, run, api
}

func liveTokenOf(t *testing.T) string {
	t.Helper()
	c, err := claudecode.ReadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	return c.OAuth.AccessToken
}

func TestUsageCachePolicy(t *testing.T) {
	_, _, api := twoAccounts(t, "")
	p, _ := store.Load("beta")

	if rd := readUsage(t.Context(), p, false, false); rd.err != nil || rd.report == nil {
		t.Fatalf("first read: %+v", rd)
	}
	readUsage(t.Context(), p, false, false) // within the idle interval: cached
	if api.usageHits["beta"] != 1 {
		t.Errorf("usage requests = %d, want 1 (second read should be cached)", api.usageHits["beta"])
	}
	readUsage(t.Context(), p, false, true) // force
	if api.usageHits["beta"] != 2 {
		t.Errorf("forced read did not fetch: %d", api.usageHits["beta"])
	}

	// A 429 keeps the last reading (with an error) and backs off.
	api.usageStatus["beta"] = 429
	rd := readUsage(t.Context(), p, false, true)
	if rd.report == nil || rd.err == nil || !strings.Contains(rd.err.Error(), "rate") {
		t.Errorf("429 read = %+v", rd)
	}
	hits := api.usageHits["beta"]
	readUsage(t.Context(), p, false, true)
	if api.usageHits["beta"] != hits {
		t.Error("fetched again during the backoff")
	}
	saveUsageCache()
	if _, err := os.Stat(usageCachePath()); err != nil {
		t.Errorf("cache not saved: %v", err)
	}
}

func TestPollInterval(t *testing.T) {
	idle, _ := usage.Parse([]byte(usageBody(10, 20, "")))
	near, _ := usage.Parse([]byte(usageBody(85, 20, "")))
	if pollInterval(idle, false) != pollIdle || pollInterval(idle, true) != pollActive || pollInterval(near, false) != pollActive {
		t.Error("poll intervals wrong")
	}
}

const cedar = `,"cedar_ember":{"eligible":true,"at_limit":true,"grants":[{"id":"example-grant","label":"Fable reset","resets_total":1,"resets_left":1,"ends_at":"2030-03-01T00:00:00Z","usable_now":true,"use_requires_limit":true}],"next_grant_id":"example-grant"}`

func TestResetsListAndClaim(t *testing.T) {
	_, run, api := twoAccounts(t, "")
	api.usage["alpha"] = usageBody(100, 100, cedar)

	out := captureStdout(t, func() {
		if err := run("resets", "alpha"); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Fable reset") || !strings.Contains(out, "1/1") {
		t.Errorf("resets output = %q", out)
	}

	if err := run("reset", "alpha", "-y"); err != nil {
		t.Fatal(err)
	}
	if len(api.claims) != 1 {
		t.Fatalf("claims = %v", api.claims)
	}
	c := api.claims[0]
	if c["program"] != "cedar_ember" || c["grant_id"] != "example-grant" || c["request_id"] == "" {
		t.Errorf("claim body = %v", c)
	}
	if api.claimPaths[0] != "/api/organizations/org-"+uuidAlpha[:8]+"/reset_rate_limits" {
		t.Errorf("claim path = %s", api.claimPaths[0])
	}

	// Unknown grant, and a grant that needs a limit while under it, are refused locally.
	if err := run("reset", "alpha", "nope", "-y"); err == nil {
		t.Error("unknown grant accepted")
	}
	api.usage["alpha"] = usageBody(10, 10, strings.Replace(cedar, `"usable_now":true`, `"usable_now":false`, 1))
	forgetUsage(&store.Profile{AccountUUID: uuidAlpha})
	if err := run("reset", "alpha", "-y"); err == nil {
		t.Error("claimed a reset that needs a limit while under it")
	}
	if len(api.claims) != 1 {
		t.Errorf("refused claims were sent: %v", api.claims)
	}

	// A server refusal is reported as an error.
	api.usage["alpha"] = usageBody(100, 100, cedar)
	api.claimResponse = `{"result":"cooldown","reason":"too_soon"}`
	forgetUsage(&store.Profile{AccountUUID: uuidAlpha})
	if err := run("reset", "alpha", "-y"); err == nil || !strings.Contains(err.Error(), "cooldown") {
		t.Errorf("refusal = %v", err)
	}
}

func TestAutoSwitch(t *testing.T) {
	_, _, api := twoAccounts(t, "alpha")
	api.usage["alpha"] = usageBody(95, 40, "")
	api.usage["beta"] = usageBody(10, 20, "")
	o := autoOptions{Threshold: 90, WeeklyThreshold: 90, Cooldown: time.Minute}

	// Dry run: nothing changes.
	o.DryRun = true
	if err := autoTick(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if liveTokenOf(t) != "dummy-alpha" {
		t.Fatal("dry run switched")
	}

	o.DryRun = false
	if err := autoTick(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if liveTokenOf(t) != "dummy-beta" {
		t.Fatalf("not switched to beta: %s", liveTokenOf(t))
	}

	// Cooldown: beta over the line now, alpha has room, but it just switched.
	api.usage["beta"] = usageBody(97, 40, "")
	api.usage["alpha"] = usageBody(5, 5, "")
	forgetUsage(&store.Profile{AccountUUID: uuidBeta})
	forgetUsage(&store.Profile{AccountUUID: uuidAlpha})
	if err := autoTick(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if liveTokenOf(t) != "dummy-beta" {
		t.Error("switched during the cooldown")
	}
}

func TestAutoAllExhausted(t *testing.T) {
	_, _, api := twoAccounts(t, "alpha")
	api.usage["alpha"] = usageBody(95, 40, "")
	api.usage["beta"] = usageBody(99, 40, "")
	if err := autoTick(t.Context(), autoOptions{Threshold: 90, WeeklyThreshold: 90}); err != nil {
		t.Fatal(err)
	}
	if liveTokenOf(t) != "dummy-alpha" {
		t.Error("switched onto an exhausted account")
	}
}

func TestPressure(t *testing.T) {
	o := autoOptions{Threshold: 90, WeeklyThreshold: 97, Models: []string{"fable"}}
	r, _ := usage.Parse([]byte(`{"limits":[{"kind":"session","percent":45},{"kind":"weekly_all","percent":96},{"kind":"weekly_scoped","percent":98,"scope":{"model":{"display_name":"Fable"}}}]}`))
	if p := pressure(r, o); p < 1 {
		t.Errorf("Fable at 98%% over a 97%% line should count: %v", p)
	}
	o.Models = nil
	if p := pressure(r, o); p >= 1 {
		t.Errorf("without --model, Fable must not count: %v", p)
	}
	if pressure(nil, o) < 1 {
		t.Error("unknown usage must not look free")
	}
}

func TestSessionMode(t *testing.T) {
	_, run, _ := twoAccounts(t, "beta")
	shared := os.Getenv("CSWITCH_CLAUDE_HOME")
	os.MkdirAll(filepath.Join(shared, "projects"), 0o700)
	os.WriteFile(filepath.Join(shared, "settings.json"), []byte(`{"theme":"dark"}`), 0o600)
	cfg, _ := os.ReadFile(os.Getenv("CSWITCH_CLAUDE_JSON"))
	var m map[string]any
	json.Unmarshal(cfg, &m)
	m["mcpServers"] = map[string]any{"x": map[string]any{"command": "x"}}
	data, _ := json.Marshal(m)
	os.WriteFile(os.Getenv("CSWITCH_CLAUDE_JSON"), data, 0o600)

	p, _ := store.Load("alpha")
	home := sessionHome(p)
	if err := prepareSession(p, home); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"projects", "settings.json"} {
		if target, err := os.Readlink(filepath.Join(home.Dir, name)); err != nil || target != filepath.Join(shared, name) {
			t.Errorf("%s not shared: %q %v", name, target, err)
		}
	}
	_, acct, err := home.ReadAccount()
	if err != nil || acct.AccountUUID != uuidAlpha {
		t.Errorf("session account = %+v, %v", acct, err)
	}
	scfg, _ := os.ReadFile(home.ConfigPath())
	if !strings.Contains(string(scfg), `"mcpServers"`) {
		t.Error("MCP servers not mirrored into the session")
	}
	c, err := home.ReadCredentials()
	if err != nil || c.OAuth.AccessToken != "dummy-alpha" {
		t.Fatalf("session credentials = %+v, %v", c, err)
	}
	if liveTokenOf(t) != "dummy-beta" {
		t.Error("the global login changed")
	}

	// While the session runs, the global login may not move onto alpha.
	pidFile := filepath.Join(pidDir(home), fmt.Sprint(os.Getpid()))
	os.WriteFile(pidFile, nil, 0o600)
	if !sessionRunning(p) {
		t.Fatal("running session not detected")
	}
	if err := run("use", "alpha", "--code-only"); err == nil || !strings.Contains(err.Error(), "session") {
		t.Errorf("switch onto a running session's account: %v", err)
	}
	os.Remove(pidFile)

	// The session refreshed its token; it is saved back when the session ends.
	rotated, _ := c.WithTokens("dummy-alpha2", "dummy-r-alpha2", time.Now().Add(time.Hour))
	home.WriteCredentials(rotated)
	if _, err := adoptSessionLogin(p, home); err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Secret("alpha", "code")
	if !strings.Contains(raw, "dummy-r-alpha2") {
		t.Error("rotated session token not saved back")
	}
}

func TestMappings(t *testing.T) {
	_, run, _ := twoAccounts(t, "")
	work := t.TempDir()
	sub := filepath.Join(work, "a", "b")
	os.MkdirAll(sub, 0o755)
	if err := run("map", "alpha", work); err != nil {
		t.Fatal(err)
	}
	cleanSub, _ := store.CleanDir(sub)
	if name, _ := store.LoadMappings().Lookup(cleanSub); name != "alpha" {
		t.Errorf("subdirectory lookup = %q", name)
	}
	if name, _ := store.LoadMappings().Lookup("/"); name != "" {
		t.Errorf("unmapped lookup = %q", name)
	}
	if err := run("rename", "alpha", "first"); err != nil {
		t.Fatal(err)
	}
	if name, _ := store.LoadMappings().Lookup(cleanSub); name != "first" {
		t.Errorf("mapping not renamed: %q", name)
	}
	if err := run("unmap", work); err != nil {
		t.Fatal(err)
	}
	if len(store.LoadMappings()) != 0 {
		t.Error("unmap left the mapping")
	}
	run("map", "beta", work)
	run("remove", "beta", "-y")
	if len(store.LoadMappings()) != 0 {
		t.Error("removing a profile left its mapping")
	}
	if err := run("map", "nobody", work); err == nil {
		t.Error("mapped an unknown account")
	}
}

func TestServiceUnits(t *testing.T) {
	t.Setenv("CSWITCH_HOME", "/tmp/cs home")
	plist := launchAgentPlist("/usr/local/bin/cswitch", []string{"auto", "--threshold=80", "--model=A&B"})
	for _, want := range []string{"<string>/usr/local/bin/cswitch</string>", "<string>--threshold=80</string>", "<string>--model=A&amp;B</string>", serviceLabel, "/tmp/cs home/auto.log", "<key>KeepAlive</key>"} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist lacks %q", want)
		}
	}
	unit := systemdUnit("/usr/bin/cswitch", []string{"auto", "--interval=2m0s"})
	if !strings.Contains(unit, `ExecStart="/usr/bin/cswitch" "auto" "--interval=2m0s"`) || !strings.Contains(unit, "Restart=always") {
		t.Errorf("unit = %s", unit)
	}
}
