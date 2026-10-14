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
	"github.com/dollarkillerx/my-claude/internal/i18n"
)

// The Claude Code flow with file-based credentials (as on Linux) and a key
// file instead of the keychain. It touches no keychain, so it runs everywhere.
func TestCodeSwitchWithFileBackend(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, ".claude", ".credentials.json")
	t.Setenv("CSWITCH_HOME", filepath.Join(dir, "home"))
	t.Setenv("CSWITCH_CLAUDE_JSON", filepath.Join(dir, ".claude.json"))
	t.Setenv("CSWITCH_CODE_CREDENTIALS_FILE", credFile)
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(dir, "home", "master.key"))
	t.Setenv("CSWITCH_DESKTOP_DIR", filepath.Join(dir, "no-desktop"))
	t.Setenv("CSWITCH_DESKTOP_APP", "cswitch-no-such-app")
	t.Setenv("CSWITCH_CLAUDE_PROC", "cswitch-no-such-proc")
	i18n.SetLang("en")

	login := func(who, uuid string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(credFile), 0o700)
		creds := fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"dummy-%s","refreshToken":"dummy-r-%s","expiresAt":%d,"scopes":[],"subscriptionType":"pro"}}`,
			who, who, time.Now().Add(time.Hour).UnixMilli())
		os.WriteFile(credFile, []byte(creds), 0o600)
		cfg := fmt.Sprintf(`{"keep":1,"oauthAccount":{"accountUuid":%q,"emailAddress":"%s@example.com"}}`, uuid, who)
		os.WriteFile(os.Getenv("CSWITCH_CLAUDE_JSON"), []byte(cfg), 0o600)
	}
	run := func(args ...string) error {
		root := NewRoot()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		return root.Execute()
	}
	must := func(args ...string) {
		t.Helper()
		if err := run(args...); err != nil {
			t.Fatalf("cswitch %s: %v", strings.Join(args, " "), err)
		}
	}
	token := func() string {
		t.Helper()
		c, err := claudecode.ReadCredentials()
		if err != nil {
			t.Fatal(err)
		}
		return c.OAuth.AccessToken
	}

	login("alice", "aaaaaaaa-0000-0000-0000-000000000001")
	must("add", "a", "--code-only")
	login("bob", "bbbbbbbb-0000-0000-0000-000000000002")
	must("add", "b", "--code-only")

	must("use", "a", "--code-only")
	if token() != "dummy-alice" {
		t.Errorf("token after use a = %s", token())
	}
	if fi, _ := os.Stat(credFile); fi.Mode().Perm() != 0o600 {
		t.Errorf("credentials file mode = %v", fi.Mode().Perm())
	}
	if b, _ := os.ReadFile(os.Getenv("CSWITCH_CLAUDE_JSON")); !strings.Contains(string(b), "alice@example.com") || !strings.Contains(string(b), `"keep": 1`) {
		t.Errorf("~/.claude.json = %s", b)
	}
	must("use", "b", "--code-only")
	if token() != "dummy-bob" {
		t.Errorf("token after use b = %s", token())
	}

	// Saved secrets are encrypted with the key file, never stored in clear.
	if fi, err := os.Stat(os.Getenv("CSWITCH_KEY_FILE")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("key file: %v %v", fi, err)
	}
	enc, _ := os.ReadFile(filepath.Join(dir, "home", "profiles", "a", "code.enc"))
	if len(enc) == 0 || strings.Contains(string(enc), "dummy-alice") {
		t.Error("profile secret missing or not encrypted")
	}

	// Rolling back a failed switch when there was no login before removes the file again.
	if os.Geteuid() == 0 {
		t.Skip("permission-based failure injection does not work as root")
	}
	os.Remove(credFile)
	os.Chmod(filepath.Dir(os.Getenv("CSWITCH_CLAUDE_JSON")), 0o500) // ~/.claude.json can't be replaced
	err := run("use", "a", "--code-only")
	os.Chmod(filepath.Dir(os.Getenv("CSWITCH_CLAUDE_JSON")), 0o700)
	if err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(credFile); !os.IsNotExist(err) {
		t.Error("credentials written by the failed switch were not removed")
	}
}
