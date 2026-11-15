package claudecode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/keychain"
)

// Home is one Claude Code configuration home.
//
// The default home is ~/.claude with ~/.claude.json and, on macOS, the
// "Claude Code-credentials" keychain item. A home with Dir set is what Claude
// Code uses when started with CLAUDE_CONFIG_DIR=Dir (cswitch's session mode):
// its global config is Dir/.claude.json and its credentials are the keychain
// item "Claude Code-credentials-<first 8 hex of SHA-256(Dir)>" on macOS, or
// Dir/.credentials.json elsewhere. Claude Code hashes the exported string as
// is, so Dir must be passed to claude exactly as given here.
type Home struct{ Dir string }

// Default is the home Claude Code uses without CLAUDE_CONFIG_DIR.
var Default = Home{}

// IsDefault reports whether h is the default home.
func (h Home) IsDefault() bool { return h.Dir == "" }

// ConfigDir is the directory holding settings, history and lock files.
// CSWITCH_CLAUDE_HOME overrides the default one for testing.
func (h Home) ConfigDir() string {
	if !h.IsDefault() {
		return h.Dir
	}
	if p := os.Getenv("CSWITCH_CLAUDE_HOME"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// ConfigPath is the global config file (~/.claude.json by default).
func (h Home) ConfigPath() string {
	if h.IsDefault() {
		return ConfigPath()
	}
	return filepath.Join(h.Dir, ".claude.json")
}

// keychainService is the keychain item holding this home's credentials.
func (h Home) keychainService() string {
	if h.IsDefault() {
		return KeychainService()
	}
	sum := sha256.Sum256([]byte(h.Dir))
	return KeychainService() + "-" + hex.EncodeToString(sum[:])[:8]
}

// credentialsFile is this home's credentials file, or "" when the keychain
// holds them (macOS).
func (h Home) credentialsFile() string {
	if h.IsDefault() {
		return credentialsFile()
	}
	if runtime.GOOS == "darwin" && os.Getenv("CSWITCH_CODE_CREDENTIALS_FILE") == "" {
		return ""
	}
	return filepath.Join(h.Dir, ".credentials.json")
}

// ReadCredentials returns this home's credentials, or ErrNotLoggedIn.
func (h Home) ReadCredentials() (*Credentials, error) {
	if path := h.credentialsFile(); path != "" {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		if err != nil {
			return nil, err
		}
		return ParseCredentials(string(data))
	}
	raw, err := keychain.Get(h.keychainService(), "")
	if errors.Is(err, keychain.ErrNotFound) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	return ParseCredentials(raw)
}

// WriteCredentials replaces this home's credentials. On macOS an existing
// keychain item's account name is reused so Claude Code keeps finding it.
func (h Home) WriteCredentials(c *Credentials) error {
	if path := h.credentialsFile(); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		return fsutil.AtomicWrite(path, []byte(c.Raw), 0o600)
	}
	acct, err := keychain.Account(h.keychainService())
	if errors.Is(err, keychain.ErrNotFound) {
		u, uerr := user.Current()
		if uerr != nil {
			return uerr
		}
		acct = u.Username
	} else if err != nil {
		return err
	}
	return keychain.Set(h.keychainService(), acct, c.Raw)
}

// DeleteCredentials removes this home's credentials; missing ones are fine.
func (h Home) DeleteCredentials() error {
	if path := h.credentialsFile(); path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	acct, err := keychain.Account(h.keychainService())
	if errors.Is(err, keychain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return keychain.Delete(h.keychainService(), acct)
}

// ReadAccount returns oauthAccount from this home's config, raw and parsed.
func (h Home) ReadAccount() (json.RawMessage, *Account, error) {
	m, err := fsutil.ReadJSONObject(h.ConfigPath())
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

// WriteAccount replaces oauthAccount in this home's config, leaving every
// other key alone, under Claude Code's config lock.
func (h Home) WriteAccount(raw json.RawMessage, backupDir string) error {
	return h.WithConfigLock(func() error {
		path := h.ConfigPath()
		m, err := fsutil.ReadJSONObject(path)
		if os.IsNotExist(err) {
			m = map[string]json.RawMessage{} // Claude Code creates it on first run
		} else if err != nil {
			return err
		}
		if backupDir != "" {
			if err := fsutil.Backup(path, backupDir); err != nil {
				return i18n.E("err.backup", path, err)
			}
		}
		m["oauthAccount"] = raw
		return fsutil.WriteJSONObject(path, m)
	})
}
