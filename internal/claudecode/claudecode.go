// Package claudecode reads and writes Claude Code's login state:
// the "Claude Code-credentials" keychain item and oauthAccount in ~/.claude.json.
package claudecode

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/keychain"
)

// KeychainService is the item Claude Code stores its OAuth credentials in.
// CSWITCH_CODE_KEYCHAIN_SERVICE overrides it for testing.
func KeychainService() string {
	if s := os.Getenv("CSWITCH_CODE_KEYCHAIN_SERVICE"); s != "" {
		return s
	}
	return "Claude Code-credentials"
}

// credentialsFile is where Claude Code keeps its credentials on platforms
// without the macOS keychain (~/.claude/.credentials.json), or "" on macOS.
// CSWITCH_CODE_CREDENTIALS_FILE forces the file backend, for testing.
func credentialsFile() string {
	if p := os.Getenv("CSWITCH_CODE_CREDENTIALS_FILE"); p != "" {
		return p
	}
	if runtime.GOOS == "darwin" {
		return ""
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".credentials.json")
}

// ConfigPath is ~/.claude.json (CSWITCH_CLAUDE_JSON overrides it for testing).
func ConfigPath() string {
	if p := os.Getenv("CSWITCH_CLAUDE_JSON"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

type notLoggedIn struct{}

func (notLoggedIn) Error() string { return i18n.T("err.code_not_logged_in") }

// ErrNotLoggedIn means Claude Code has no stored credentials.
var ErrNotLoggedIn error = notLoggedIn{}

// procName is the executable name of the claude CLI (CSWITCH_CLAUDE_PROC
// overrides it for testing). It is lowercase, unlike the Desktop app "Claude".
func procName() string {
	if p := os.Getenv("CSWITCH_CLAUDE_PROC"); p != "" {
		return p
	}
	return "claude"
}

// Running reports whether a claude CLI process that uses Claude Code's own
// login is running. Such processes hold their OAuth tokens in memory and
// rewrite ~/.claude.json.
func Running() bool {
	// -A -o works with both BSD (macOS) and procps (Linux) ps. On Linux,
	// comm is the bare process name rather than a path, which is all we need.
	exes, err := exec.Command("ps", "-A", "-o", "comm=").Output()
	if err != nil {
		return false
	}
	cmds, err := exec.Command("ps", "-A", "-o", "args=").Output()
	if err != nil {
		return false
	}
	return runningIn(string(exes), string(cmds), procName())
}

// runningIn scans ps output for the claude CLI: exes holds one executable
// path per line (ps -o comm, which keeps spaces in paths intact), cmds one
// full command line per line.
//
// Claude Desktop bundles its own claude for the Code tab
// (…/claude.app/Contents/MacOS/claude); those processes get their login from
// Desktop, not from Claude Code's keychain item, so they are ignored. An npm
// install runs as node with the claude-code package's cli.js.
func runningIn(exes, cmds, name string) bool {
	for _, exe := range strings.Split(exes, "\n") {
		exe = strings.TrimSpace(exe)
		if exe == "" || strings.Contains(exe, ".app/Contents/MacOS/") {
			continue
		}
		if filepath.Base(exe) == name {
			return true
		}
	}
	for _, line := range strings.Split(cmds, "\n") {
		if isNpmClaude(strings.Fields(line)) {
			return true
		}
	}
	return false
}

// isNpmClaude reports whether a command line is node running the claude-code
// package's entry point (an npm install of Claude Code).
func isNpmClaude(args []string) bool {
	if len(args) < 2 || filepath.Base(args[0]) != "node" {
		return false
	}
	for _, a := range args[1:] {
		if strings.HasSuffix(a, "@anthropic-ai/claude-code/cli.js") || strings.HasSuffix(a, "@anthropic-ai/claude-code/cli.mjs") {
			return true
		}
		if !strings.HasPrefix(a, "-") { // the script is the first non-flag argument
			return false
		}
	}
	return false
}

// OAuth is the subset of the credentials JSON cswitch needs to understand.
type OAuth struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // unix milliseconds
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType"`
}

// Credentials is the raw credentials JSON plus its parsed OAuth section.
// Raw is what gets stored and restored, so unknown fields are preserved.
type Credentials struct {
	Raw   string
	OAuth OAuth
}

// ParseCredentials parses the keychain JSON blob.
func ParseCredentials(raw string) (*Credentials, error) {
	var v struct {
		ClaudeAiOauth *OAuth `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, i18n.E("err.code_creds_parse", err)
	}
	if v.ClaudeAiOauth == nil || v.ClaudeAiOauth.RefreshToken == "" {
		return nil, i18n.E("err.code_creds_no_oauth")
	}
	return &Credentials{Raw: raw, OAuth: *v.ClaudeAiOauth}, nil
}

// Expired reports whether the access token expires within margin.
func (c *Credentials) Expired(margin time.Duration) bool {
	return time.Now().Add(margin).UnixMilli() >= c.OAuth.ExpiresAt
}

// WithTokens returns a copy of the credentials with new tokens, keeping all other fields.
func (c *Credentials) WithTokens(access, refresh string, expiresAt time.Time) (*Credentials, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(c.Raw), &outer); err != nil {
		return nil, err
	}
	var inner map[string]any
	if err := json.Unmarshal(outer["claudeAiOauth"], &inner); err != nil {
		return nil, err
	}
	inner["accessToken"] = access
	if refresh != "" {
		inner["refreshToken"] = refresh
	}
	inner["expiresAt"] = expiresAt.UnixMilli()
	b, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	outer["claudeAiOauth"] = b
	raw, err := json.Marshal(outer)
	if err != nil {
		return nil, err
	}
	return ParseCredentials(string(raw))
}

// ReadCredentials returns the live Claude Code credentials.
func ReadCredentials() (*Credentials, error) {
	if path := credentialsFile(); path != "" {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		if err != nil {
			return nil, err
		}
		return ParseCredentials(string(data))
	}
	raw, err := keychain.Get(KeychainService(), "")
	if errors.Is(err, keychain.ErrNotFound) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	return ParseCredentials(raw)
}

// WriteCredentials replaces the live Claude Code credentials. On macOS the
// existing keychain item's account name is reused so Claude Code keeps
// finding it.
func WriteCredentials(c *Credentials) error {
	if path := credentialsFile(); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		return fsutil.AtomicWrite(path, []byte(c.Raw), 0o600)
	}
	acct, err := keychain.Account(KeychainService())
	if errors.Is(err, keychain.ErrNotFound) {
		u, uerr := user.Current()
		if uerr != nil {
			return uerr
		}
		acct = u.Username
	} else if err != nil {
		return err
	}
	return keychain.Set(KeychainService(), acct, c.Raw)
}

// DeleteCredentials removes Claude Code's credentials (used to undo a write
// when Claude Code had no credentials before).
func DeleteCredentials() error {
	if path := credentialsFile(); path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	acct, err := keychain.Account(KeychainService())
	if errors.Is(err, keychain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return keychain.Delete(KeychainService(), acct)
}

// Account is the subset of oauthAccount shown to the user.
type Account struct {
	AccountUUID      string `json:"accountUuid"`
	EmailAddress     string `json:"emailAddress"`
	DisplayName      string `json:"displayName"`
	OrganizationUUID string `json:"organizationUuid"`
	OrganizationName string `json:"organizationName"`
	BillingType      string `json:"billingType"`
}

// ReadAccount returns oauthAccount from ~/.claude.json, raw and parsed.
func ReadAccount() (json.RawMessage, *Account, error) {
	m, err := fsutil.ReadJSONObject(ConfigPath())
	if err != nil {
		return nil, nil, err
	}
	raw, ok := m["oauthAccount"]
	if !ok || string(raw) == "null" {
		return nil, nil, i18n.E("err.code_no_oauth_account")
	}
	var a Account
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, nil, i18n.E("err.code_oauth_account_parse", err)
	}
	if a.AccountUUID == "" {
		return nil, nil, i18n.E("err.code_oauth_account_parse", errors.New("accountUuid"))
	}
	return raw, &a, nil
}

// WriteAccount replaces oauthAccount in ~/.claude.json, leaving every other key alone.
func WriteAccount(raw json.RawMessage, backupDir string) error {
	path := ConfigPath()
	m, err := fsutil.ReadJSONObject(path)
	if os.IsNotExist(err) {
		m = map[string]json.RawMessage{} // Claude Code creates it on first run
	} else if err != nil {
		return err
	}
	if err := fsutil.Backup(path, backupDir); err != nil {
		return i18n.E("err.backup", path, err)
	}
	m["oauthAccount"] = raw
	return fsutil.WriteJSONObject(path, m)
}
