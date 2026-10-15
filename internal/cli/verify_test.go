package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/desktop"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

const (
	uuidAlpha = "1a2b3c4d-0000-0000-0000-000000000001"
	uuidBeta  = "5e6f7a8b-0000-0000-0000-000000000002"
)

// fileEnv sets up a sandbox with file-based Claude Code credentials and a key
// file, so no keychain is touched.
func fileEnv(t *testing.T) (credFile string, run func(args ...string) error) {
	t.Helper()
	dir := t.TempDir()
	credFile = filepath.Join(dir, ".claude", ".credentials.json")
	t.Setenv("CSWITCH_HOME", filepath.Join(dir, "home"))
	t.Setenv("CSWITCH_CLAUDE_JSON", filepath.Join(dir, ".claude.json"))
	t.Setenv("CSWITCH_CODE_CREDENTIALS_FILE", credFile)
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(dir, "home", "master.key"))
	t.Setenv("CSWITCH_DESKTOP_DIR", filepath.Join(dir, "no-desktop"))
	t.Setenv("CSWITCH_DESKTOP_APP", "cswitch-no-such-app")
	t.Setenv("CSWITCH_CLAUDE_PROC", "cswitch-no-such-proc")
	i18n.SetLang("en")
	os.MkdirAll(filepath.Dir(credFile), 0o700)
	run = func(args ...string) error {
		root := NewRoot()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		return root.Execute()
	}
	return credFile, run
}

func writeCreds(t *testing.T, path, who string, expires time.Time) {
	t.Helper()
	c := fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"dummy-%s","refreshToken":"dummy-r-%s","expiresAt":%d,"scopes":[],"subscriptionType":"max"}}`, who, who, expires.UnixMilli())
	if err := os.WriteFile(path, []byte(c), 0o600); err != nil {
		t.Fatal(err)
	}
	resetLiveCache()
}

func writeClaudeJSON(t *testing.T, uuid, email string) {
	t.Helper()
	cfg := fmt.Sprintf(`{"keep":1,"oauthAccount":{"accountUuid":%q,"emailAddress":%q}}`, uuid, email)
	os.WriteFile(os.Getenv("CSWITCH_CLAUDE_JSON"), []byte(cfg), 0o600)
}

// The bug the user hit: the CLI's keychain held alpha's token while
// ~/.claude.json (rewritten by Desktop's embedded Claude Code) said beta.
// The token must be saved as the account the server says it belongs to.
func TestTokenLabeledByServerNotClaudeJSON(t *testing.T) {
	credFile, run := fileEnv(t)
	newFakeAPI(t, map[string][2]string{"alpha": {uuidAlpha, "alpha@example.com"}})
	writeCreds(t, credFile, "alpha", time.Now().Add(time.Hour))
	writeClaudeJSON(t, uuidBeta, "beta@example.com") // stale / Desktop's account

	if err := run("add", "--code-only"); err != nil {
		t.Fatal(err)
	}
	p, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("token not saved as alpha: %v", err)
	}
	if p.AccountUUID != uuidAlpha || p.Email != "alpha@example.com" {
		t.Errorf("profile = %+v", p)
	}
	if _, err := store.Load("beta"); err == nil {
		t.Error("alpha's token was saved under beta's name")
	}
	// ~/.claude.json is put right so Claude Code shows the right account.
	_, acct, _ := claudecode.ReadAccount()
	if acct.AccountUUID != uuidAlpha {
		t.Errorf("~/.claude.json still names %s", acct.EmailAddress)
	}
}

// Profiles saved by the earlier version with a wrong label are repaired the
// next time usage is checked.
func TestUsageRepairsMislabeledProfile(t *testing.T) {
	credFile, run := fileEnv(t)
	api := newFakeAPI(t, map[string][2]string{"alpha": {uuidAlpha, "alpha@example.com"}, "alpha2": {uuidAlpha, "alpha@example.com"}})
	_ = api

	// "beta" holds a token that belongs to alpha; "alpha" is Desktop-only.
	store.Save(&store.Profile{Name: "beta", AccountUUID: uuidBeta, Email: "beta@example.com", HasCode: true, HasDesktop: true})
	creds := fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"dummy-alpha2","refreshToken":"r2","expiresAt":%d}}`, time.Now().Add(time.Hour).UnixMilli())
	store.SetSecret("beta", "code", creds)
	store.Save(&store.Profile{Name: "alpha", AccountUUID: uuidAlpha, HasDesktop: true})
	os.Remove(credFile) // Claude Code logged out: neither profile is live

	if err := run("usage"); err != nil {
		t.Fatal(err)
	}
	y, _ := store.Load("beta")
	if y.HasCode || !y.HasDesktop {
		t.Errorf("beta = code %v desktop %v, want only its Desktop login", y.HasCode, y.HasDesktop)
	}
	if _, err := store.Secret("beta", "code"); err == nil {
		t.Error("wrong token still stored in beta")
	}
	a, _ := store.Load("alpha")
	if !a.HasCode || a.Email != "alpha@example.com" {
		t.Errorf("token not moved to alpha: %+v", a)
	}
	if got, _ := store.Secret("alpha", "code"); !strings.Contains(got, "dummy-alpha2") {
		t.Error("alpha did not receive the token")
	}
}

// fetchUsage must not return while a query can still write credentials.
func TestFetchUsageWaitsForQueries(t *testing.T) {
	credFile, _ := fileEnv(t)
	api := newFakeAPI(t, map[string][2]string{"bob": {uuidBeta, "bob@example.com"}, "new": {uuidBeta, "bob@example.com"}})
	writeCreds(t, credFile, "bob", time.Now().Add(-time.Hour)) // expired: usage will refresh
	writeClaudeJSON(t, uuidBeta, "bob@example.com")
	store.Save(&store.Profile{Name: "bob", AccountUUID: uuidBeta, HasCode: true})
	raw, _ := os.ReadFile(credFile)
	store.SetSecret("bob", "code", string(raw))

	// Slow token endpoint: slower than the picker's timeout.
	slow := api.srv.Config.Handler
	api.srv.Config.Handler = http_slow(slow, 300*time.Millisecond)

	p, _ := store.Load("bob")
	start := time.Now()
	fetchUsage(t.Context(), []*store.Profile{p}, "bob", 50*time.Millisecond)
	if time.Since(start) > 2*time.Second {
		t.Fatal("fetchUsage did not give up after its timeout")
	}
	before, _ := os.ReadFile(credFile)
	time.Sleep(500 * time.Millisecond) // let the slow server finish
	after, _ := os.ReadFile(credFile)
	if !bytes.Equal(before, after) {
		t.Error("credentials were written after fetchUsage returned")
	}
}

func TestAddWithNothingLoggedIn(t *testing.T) {
	_, run := fileEnv(t)
	newFakeAPI(t, nil)
	want := "nothing was saved"
	if !desktop.Supported() {
		want = "claude auth login" // no Desktop: only Claude Code can be saved
	}
	if err := run("add"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v", err)
	}
}

func TestReservedNamesInCommands(t *testing.T) {
	credFile, run := fileEnv(t)
	newFakeAPI(t, map[string][2]string{"x": {uuidAlpha, "sync@example.com"}})
	writeCreds(t, credFile, "x", time.Now().Add(time.Hour))
	if err := run("add", "sync"); err == nil {
		t.Error("a command name was accepted as an account name")
	}
	// Auto-naming from sync@example.com must not produce "sync".
	if err := run("add", "--code-only"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("sync"); err == nil {
		t.Error("auto-named profile shadows the sync command")
	}
	if _, err := store.Load("sync-2"); err != nil {
		t.Errorf("expected sync-2: %v", err)
	}
}

// login <name> for an account that is already saved keeps the existing name.
func TestSaveCodeKeepsExistingNameForLogin(t *testing.T) {
	credFile, _ := fileEnv(t)
	newFakeAPI(t, map[string][2]string{"alpha": {uuidAlpha, "alpha@example.com"}})
	writeCreds(t, credFile, "alpha", time.Now().Add(time.Hour))
	if _, err := saveCode("alpha", true); err != nil {
		t.Fatal(err)
	}
	p, err := saveCode("work", false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "alpha" {
		t.Errorf("saved as %q, want existing alpha", p.Name)
	}
	if _, err := store.Load("work"); err == nil {
		t.Error("existing profile renamed to work")
	}
}
