package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithTokensKeepsUnknownFields(t *testing.T) {
	c, err := ParseCredentials(`{"claudeAiOauth":{"accessToken":"a1","refreshToken":"r1","expiresAt":1,"scopes":["user:inference"],"subscriptionType":"max","rateLimitTier":"x"},"mcpOAuth":{"k":"v"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Expired(0) {
		t.Error("expected expired")
	}
	exp := time.Now().Add(time.Hour)
	n, err := c.WithTokens("a2", "r2", exp)
	if err != nil {
		t.Fatal(err)
	}
	if n.OAuth.AccessToken != "a2" || n.OAuth.RefreshToken != "r2" || n.OAuth.ExpiresAt != exp.UnixMilli() || n.OAuth.SubscriptionType != "max" {
		t.Errorf("oauth = %+v", n.OAuth)
	}
	var m map[string]map[string]any
	json.Unmarshal([]byte(n.Raw), &m)
	if m["mcpOAuth"]["k"] != "v" || m["claudeAiOauth"]["rateLimitTier"] != "x" {
		t.Errorf("unknown fields lost: %s", n.Raw)
	}
}

func TestParseCredentialsRejectsAPIKeyLogin(t *testing.T) {
	if _, err := ParseCredentials(`{"mcpOAuth":{}}`); err == nil {
		t.Error("expected error")
	}
}

func TestWriteAccountPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	os.WriteFile(path, []byte(`{"numStartups":5,"oauthAccount":{"accountUuid":"old"},"projects":{"/x":{"a":1}}}`), 0o600)
	t.Setenv("CSWITCH_CLAUDE_JSON", path)

	if err := WriteAccount(json.RawMessage(`{"accountUuid":"new","emailAddress":"b@example.com"}`), filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	_, a, err := ReadAccount()
	if err != nil {
		t.Fatal(err)
	}
	if a.AccountUUID != "new" || a.EmailAddress != "b@example.com" {
		t.Errorf("account = %+v", a)
	}
	var m map[string]any
	b, _ := os.ReadFile(path)
	json.Unmarshal(b, &m)
	if m["numStartups"] != float64(5) || m["projects"] == nil {
		t.Errorf("other keys lost: %s", b)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode())
	}
	if bk, _ := filepath.Glob(filepath.Join(dir, "backups", ".claude.json.*")); len(bk) != 1 {
		t.Errorf("backups = %v", bk)
	}
}

func TestRunning(t *testing.T) {
	t.Setenv("CSWITCH_CLAUDE_PROC", "cswitch-no-such-process")
	if Running() {
		t.Error("Running() true for a process that does not exist")
	}
}

func TestReadAccountErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	t.Setenv("CSWITCH_CLAUDE_JSON", path)
	for name, content := range map[string]string{
		"no oauthAccount": `{"numStartups":1}`,
		"null":            `{"oauthAccount":null}`,
		"no uuid":         `{"oauthAccount":{"emailAddress":"a@b"}}`,
		"wrong type":      `{"oauthAccount":"x"}`,
	} {
		os.WriteFile(path, []byte(content), 0o600)
		if _, _, err := ReadAccount(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestRunningIn(t *testing.T) {
	desktopClaude := "/Users/me/Library/Application Support/Claude/claude-code/2.1.0/abc/claude.app/Contents/MacOS/claude"
	cases := []struct {
		name       string
		exes, cmds string
		want       bool
	}{
		{"native CLI", "/bin/zsh\nclaude\n", "", true},
		{"full path", "/Users/me/.local/bin/claude\n", "", true},
		{"path with spaces", "/Users/me/My Tools/claude\n", "", true},
		{"only Desktop's bundled claude", "/Applications/Claude.app/Contents/MacOS/Claude\n" + desktopClaude + "\n", desktopClaude + " --output-format stream-json\n", false},
		{"Desktop app itself", "/Applications/Claude.app/Contents/MacOS/Claude\n", "", false},
		{"npm install", "node\n", "node /usr/local/lib/node_modules/@anthropic-ai/claude-code/cli.js\n", true},
		{"npm with node flags", "node\n", "/opt/homebrew/bin/node --no-warnings /x/node_modules/@anthropic-ai/claude-code/cli.js -p hi\n", true},
		{"pattern only mentioned in args", "zsh\n", "zsh -c echo @anthropic-ai/claude-code/cli.js\nvim node_modules/@anthropic-ai/claude-code/cli.js\n", false},
		{"node running something else", "node\n", "node server.js @anthropic-ai/claude-code/cli.js\n", false},
		{"similar names", "claude-helper\nclaudes\n", "", false},
		{"nothing", "", "", false},
	}
	for _, c := range cases {
		if got := runningIn(c.exes, c.cmds, "claude"); got != c.want {
			t.Errorf("%s: runningIn = %v, want %v", c.name, got, c.want)
		}
	}
}
