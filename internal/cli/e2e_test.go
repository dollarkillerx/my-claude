//go:build e2e

// End-to-end tests for the commands. They run against a sandbox (temporary
// config files, a fake Desktop directory, and test-only keychain items holding
// dummy values), but they do use the real macOS keychain, so they are behind
// the e2e build tag:
//
//	go test -tags e2e ./internal/cli
package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/desktop"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/keychain"
	"github.com/dollarkillerx/my-claude/internal/store"
)

const (
	uuidA = "aaaaaaaa-0000-0000-0000-000000000001"
	uuidB = "bbbbbbbb-0000-0000-0000-000000000002"
	uuidC = "cccccccc-0000-0000-0000-000000000003"
	orgA  = "aaaaaaaa-0000-0000-0000-0000000000a1"
	orgB  = "bbbbbbbb-0000-0000-0000-0000000000b2"
)

type sandbox struct {
	t   *testing.T
	dir string
}

func newSandbox(t *testing.T) *sandbox {
	dir := t.TempDir()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	t.Setenv("CSWITCH_HOME", filepath.Join(dir, "home"))
	t.Setenv("CSWITCH_CLAUDE_JSON", filepath.Join(dir, "claude.json"))
	t.Setenv("CSWITCH_DESKTOP_DIR", filepath.Join(dir, "desktop"))
	t.Setenv("CSWITCH_CODE_KEYCHAIN_SERVICE", "cswitch-e2e-code-"+suffix)
	t.Setenv("CSWITCH_KEYCHAIN_SERVICE", "cswitch-e2e-"+suffix)
	t.Setenv("CSWITCH_DESKTOP_APP", "cswitch-e2e-no-such-app")
	t.Setenv("CSWITCH_CLAUDE_PROC", "cswitch-e2e-no-such-proc")
	i18n.SetLang("en")
	s := &sandbox{t, dir}
	t.Cleanup(func() {
		for _, name := range []string{"a", "b"} {
			store.Remove(name)
		}
		keychain.Delete(os.Getenv("CSWITCH_KEYCHAIN_SERVICE"), "master-key")
		acct, err := keychain.Account(claudecode.KeychainService())
		if err == nil {
			keychain.Delete(claudecode.KeychainService(), acct)
		}
	})
	return s
}

func creds(who string, expiresAt time.Time) string {
	return fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"dummy-%s","refreshToken":"dummy-r-%s","expiresAt":%d,"scopes":[],"subscriptionType":"max"}}`,
		who, who, expiresAt.UnixMilli())
}

// loginCode simulates Claude Code being logged in as who.
func (s *sandbox) loginCode(who, uuid string) {
	s.t.Helper()
	if err := keychain.Set(claudecode.KeychainService(), os.Getenv("USER"), creds(who, time.Now().Add(time.Hour))); err != nil {
		s.t.Fatal(err)
	}
	s.writeAccount(who, uuid)
}

func (s *sandbox) writeAccount(who, uuid string) {
	s.t.Helper()
	cfg := fmt.Sprintf(`{"numStartups":3,"oauthAccount":{"accountUuid":%q,"emailAddress":"%s@example.com","organizationName":"Org %s"}}`, uuid, who, who)
	if err := os.WriteFile(os.Getenv("CSWITCH_CLAUDE_JSON"), []byte(cfg), 0o600); err != nil {
		s.t.Fatal(err)
	}
}

// loginDesktop simulates Claude Desktop being logged in as who.
func (s *sandbox) loginDesktop(who, uuid string) {
	s.t.Helper()
	d := desktop.Dir()
	os.MkdirAll(filepath.Join(d, "Local Storage"), 0o700)
	cfg := fmt.Sprintf(`{"locale":"en","oauth:tokenCacheV2":"enc-%s","lastKnownAccountUuid":%q}`, who, uuid)
	os.WriteFile(filepath.Join(d, "config.json"), []byte(cfg), 0o600)
	os.WriteFile(filepath.Join(d, "Cookies"), []byte("cookies-"+who), 0o600)
	os.WriteFile(filepath.Join(d, "Local Storage", "data"), []byte("storage-"+who), 0o600)
}

func (s *sandbox) run(args ...string) error {
	root := NewRoot()
	root.SetArgs(args)
	root.SetOut(os.Stderr)
	return root.Execute()
}

func (s *sandbox) mustRun(args ...string) {
	s.t.Helper()
	if err := s.run(args...); err != nil {
		s.t.Fatalf("cswitch %s: %v", strings.Join(args, " "), err)
	}
}

func codeEmail(t *testing.T) string {
	t.Helper()
	_, a, err := claudecode.ReadAccount()
	if err != nil {
		t.Fatal(err)
	}
	return a.EmailAddress
}

func liveToken(t *testing.T) string {
	t.Helper()
	c, err := claudecode.ReadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	return c.OAuth.AccessToken
}

func readDesktop(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(desktop.Dir(), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// setup saves alice as "a" (Code + Desktop) and bob as "b" (Code + Desktop),
// leaving both apps on bob.
func (s *sandbox) setup() {
	s.loginCode("alice", uuidA)
	s.loginDesktop("bob", uuidB)
	s.mustRun("add", "a")                   // Desktop is bob: only noted
	s.mustRun("add", "b", "--desktop-only") // bob's Desktop login
	s.loginCode("bob", uuidB)
	s.mustRun("add", "b") // bob's Code login merges into b
	s.loginDesktop("alice", uuidA)
	s.mustRun("add", "a", "--desktop-only")
	s.loginDesktop("bob", uuidB)
}

func TestSwitchBothApps(t *testing.T) {
	s := newSandbox(t)
	s.setup()
	sessA := filepath.Join(desktop.Dir(), "claude-code-sessions", uuidA, orgA)
	sessB := filepath.Join(desktop.Dir(), "claude-code-sessions", uuidB, orgB)
	os.MkdirAll(sessA, 0o755)
	os.MkdirAll(sessB, 0o755)
	os.WriteFile(filepath.Join(sessB, "local_1.json"), []byte("{}"), 0o644)

	s.mustRun("use", "a", "--no-launch")
	if got := codeEmail(t); got != "alice@example.com" {
		t.Errorf("Code account = %s", got)
	}
	if got := liveToken(t); got != "dummy-alice" {
		t.Errorf("Code token = %s", got)
	}
	if desktop.CurrentAccount() != uuidA || readDesktop(t, "Cookies") != "cookies-alice" || readDesktop(t, "Local Storage/data") != "storage-alice" {
		t.Errorf("Desktop not switched to alice")
	}
	if _, err := os.Stat(filepath.Join(sessA, "local_1.json")); err != nil {
		t.Errorf("session from bob not shared with alice: %v", err)
	}
	if b, _ := os.ReadFile(os.Getenv("CSWITCH_CLAUDE_JSON")); !strings.Contains(string(b), `"numStartups": 3`) {
		t.Errorf("other ~/.claude.json keys lost: %s", b)
	}

	s.mustRun("use", "b", "--no-launch")
	if codeEmail(t) != "bob@example.com" || desktop.CurrentAccount() != uuidB || readDesktop(t, "Cookies") != "cookies-bob" {
		t.Errorf("not switched back to bob")
	}
}

// A refused switch must not leave one app switched and the other not.
func TestUseRefusalChangesNothing(t *testing.T) {
	s := newSandbox(t)
	s.setup()
	s.loginDesktop("carol", uuidC) // unsaved Desktop account

	err := s.run("use", "a", "--no-launch")
	if err == nil || !strings.Contains(err.Error(), "not saved") {
		t.Fatalf("expected refusal, got %v", err)
	}
	if got := codeEmail(t); got != "bob@example.com" {
		t.Errorf("Code was switched to %s despite the refusal", got)
	}
	if desktop.CurrentAccount() != uuidC {
		t.Errorf("Desktop changed despite the refusal")
	}
}

// A Desktop switch that fails halfway is rolled back, and Code is untouched.
func TestDesktopFailureRollsBack(t *testing.T) {
	s := newSandbox(t)
	s.setup()
	// Damage a's saved Desktop state so RestoreState fails after Restore succeeded.
	saved := filepath.Join(store.Dir("a"), "desktop", "Cookies")
	os.Chmod(saved, 0o000)
	defer os.Chmod(saved, 0o600)

	if err := s.run("use", "a", "--no-launch"); err == nil {
		t.Fatal("expected failure")
	}
	if desktop.CurrentAccount() != uuidB || readDesktop(t, "Cookies") != "cookies-bob" {
		t.Errorf("Desktop not rolled back: account %s", desktop.CurrentAccount())
	}
	if got := codeEmail(t); got != "bob@example.com" {
		t.Errorf("Code switched to %s although Desktop failed", got)
	}
}

// A running claude process can write a stale oauthAccount back; the keychain
// token identifies the real owner and ~/.claude.json is repaired.
func TestStaleOAuthAccountRepaired(t *testing.T) {
	s := newSandbox(t)
	s.setup()
	s.mustRun("use", "a", "--code-only")
	s.writeAccount("bob", uuidB) // stale write by an old claude session

	s.mustRun("use", "b", "--code-only")
	if got := liveToken(t); got != "dummy-bob" {
		t.Errorf("token = %s", got)
	}
	// alice's profile must still hold alice's token, not have been overwritten.
	raw, _ := store.Secret("a", "code")
	if !strings.Contains(raw, "dummy-alice") {
		t.Errorf("profile a lost its token: %s", raw)
	}
}

func TestUsageDoesNotRefreshLiveTokenUnderRunningClaude(t *testing.T) {
	s := newSandbox(t)
	s.setup() // Code is bob
	var refreshes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			refreshes++
			json.NewEncoder(w).Encode(map[string]any{"access_token": "dummy-new", "refresh_token": "dummy-r-new", "expires_in": 3600})
		case "/usage":
			if r.Header.Get("Authorization") == "Bearer dummy-expired" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"five_hour":{"utilization":10,"resets_at":"2030-01-01T00:00:00Z"}}`)
		}
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_TOKEN_URL", srv.URL+"/token")
	t.Setenv("CSWITCH_USAGE_URL", srv.URL+"/usage")

	expired := strings.Replace(creds("expired", time.Now().Add(-time.Hour)), "dummy-r-expired", "dummy-r-bob", 1)
	keychain.Set(claudecode.KeychainService(), os.Getenv("USER"), expired)

	// Stand-in for a running claude process.
	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	defer proc.Process.Kill()
	t.Setenv("CSWITCH_CLAUDE_PROC", "sleep")
	if !claudecode.Running() {
		t.Skip("cannot see the stand-in process with pgrep in this environment")
	}
	p, _ := store.Load("b")
	if _, err := profileUsage(t.Context(), p, true); err == nil || refreshes != 0 {
		t.Fatalf("live token refreshed under a running claude (err=%v, refreshes=%d)", err, refreshes)
	}

	t.Setenv("CSWITCH_CLAUDE_PROC", "cswitch-e2e-no-such-proc")
	r, err := profileUsage(t.Context(), p, true)
	if err != nil || refreshes != 1 || len(r.Windows) != 1 {
		t.Fatalf("refresh without running claude: %v, refreshes=%d", err, refreshes)
	}
	if got := liveToken(t); got != "dummy-new" {
		t.Errorf("refreshed token not written back to Claude Code: %s", got)
	}
}

// Desktop snapshots hold encrypted token blobs that can be several KB, past
// the ~2 KB limit of passing secrets through security -i.
func TestLargeDesktopLogin(t *testing.T) {
	s := newSandbox(t)
	s.setup()
	big := strings.Repeat("A", 6000)
	cfg := fmt.Sprintf(`{"locale":"en","oauth:tokenCache":"%s","oauth:tokenCacheV2":"%s","lastKnownAccountUuid":%q}`, big, big, uuidA)
	os.WriteFile(filepath.Join(desktop.Dir(), "config.json"), []byte(cfg), 0o600)
	s.mustRun("add", "a", "--desktop-only")

	s.mustRun("use", "b", "--no-launch", "--desktop-only")
	s.mustRun("use", "a", "--no-launch", "--desktop-only")
	snap, err := desktop.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snap.Keys["oauth:tokenCacheV2"]); got != `"`+big+`"` {
		t.Errorf("large token not restored intact (%d bytes)", len(got))
	}
}

// usage must find the live account by its keychain token even when a running
// claude session has written a stale account into ~/.claude.json.
func TestUsageLiveAccountByToken(t *testing.T) {
	s := newSandbox(t)
	s.setup() // Code is bob

	var refreshes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			refreshes++
			json.NewEncoder(w).Encode(map[string]any{"access_token": "dummy-new", "refresh_token": "dummy-r-new", "expires_in": 3600})
			return
		}
		fmt.Fprint(w, `{"five_hour":{"utilization":1}}`)
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_TOKEN_URL", srv.URL+"/token")
	t.Setenv("CSWITCH_USAGE_URL", srv.URL+"/usage")

	// bob's live token has expired, and the saved copy matches it.
	expired := strings.Replace(creds("bob", time.Now().Add(-time.Hour)), "dummy-bob", "dummy-expired", 1)
	keychain.Set(claudecode.KeychainService(), os.Getenv("USER"), expired)
	s.mustRun("add", "b", "--code-only")
	s.writeAccount("alice", uuidA) // stale write by an old claude session

	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	defer proc.Process.Kill()
	t.Setenv("CSWITCH_CLAUDE_PROC", "sleep")
	if !claudecode.Running() {
		t.Skip("cannot see the stand-in process in this environment")
	}

	s.mustRun("usage")
	if refreshes != 0 {
		t.Errorf("bob's live token was refreshed under a running claude (%d refreshes)", refreshes)
	}
	if got := liveToken(t); got != "dummy-expired" {
		t.Errorf("live token changed: %s", got)
	}
}
