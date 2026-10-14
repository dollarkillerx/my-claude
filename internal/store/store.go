// Package store persists cswitch profiles: metadata on disk under
// ~/.config/cswitch, secrets in the keychain under service "cswitch".
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/keychain"
)

// Profile is one saved account.
type Profile struct {
	Name             string          `json:"name"`
	AccountUUID      string          `json:"accountUuid"`
	Email            string          `json:"email"`
	OrganizationName string          `json:"organizationName,omitempty"`
	OrganizationUUID string          `json:"organizationUuid,omitempty"`
	SubscriptionType string          `json:"subscriptionType,omitempty"`
	OAuthAccount     json.RawMessage `json:"oauthAccount,omitempty"`
	HasCode          bool            `json:"hasCode"`
	HasDesktop       bool            `json:"hasDesktop"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidateName rejects names that are unsafe as directory or keychain account names.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return i18n.E("err.invalid_name", name)
	}
	return nil
}

// Home is the cswitch data directory (CSWITCH_HOME overrides it).
func Home() string {
	if h := os.Getenv("CSWITCH_HOME"); h != "" {
		return h
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cswitch")
}

// BackupDir holds copies of Claude files taken before cswitch modifies them.
func BackupDir() string { return filepath.Join(Home(), "backups") }

// Dir is the on-disk directory of a profile.
func Dir(name string) string { return filepath.Join(Home(), "profiles", name) }

func metaPath(name string) string { return filepath.Join(Dir(name), "meta.json") }

func keychainService() string {
	if s := os.Getenv("CSWITCH_KEYCHAIN_SERVICE"); s != "" {
		return s
	}
	return "cswitch"
}

// ErrNoProfile is returned by Load for unknown names.
var ErrNoProfile = errors.New("no such profile")

// Load reads a profile's metadata.
func Load(name string) (*Profile, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(metaPath(name))
	if os.IsNotExist(err) {
		return nil, i18n.E("err.no_profile", name, ErrNoProfile)
	}
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, i18n.E("err.profile_corrupt", name, err)
	}
	p.Name = name
	return &p, nil
}

// listCache memoizes List for the life of the process; commands look up
// profiles by account several times, and only Save/Remove change the set.
var (
	listCache     []*Profile
	listCacheHome string
)

// Save writes a profile's metadata.
func Save(p *Profile) error {
	listCache = nil
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(p.Name), 0o700); err != nil {
		return err
	}
	p.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(metaPath(p.Name), append(data, '\n'), 0o600)
}

// List returns all profiles sorted by name. The result is cached until the
// next Save or Remove; callers must not keep mutated profiles without saving.
func List() ([]*Profile, error) {
	if listCache != nil && listCacheHome == Home() {
		return listCache, nil
	}
	entries, err := os.ReadDir(filepath.Join(Home(), "profiles"))
	if os.IsNotExist(err) {
		return []*Profile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Profile
	for _, e := range entries {
		if !e.IsDir() || ValidateName(e.Name()) != nil {
			continue
		}
		p, err := Load(e.Name())
		if errors.Is(err, ErrNoProfile) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if out == nil {
		out = []*Profile{}
	}
	listCache, listCacheHome = out, Home()
	return out, nil
}

// FindByAccount returns the profile saved for an account UUID, or nil.
func FindByAccount(uuid string) (*Profile, error) {
	if uuid == "" {
		return nil, nil
	}
	ps, err := List()
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if p.AccountUUID == uuid {
			return p, nil
		}
	}
	return nil, nil
}

// Remove deletes a profile's files and keychain items.
func Remove(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	listCache = nil
	// Profiles saved by early versions kept their secrets in keychain items.
	for _, kind := range []string{"code", "desktop"} {
		if !useKeychain() {
			break
		}
		if err := keychain.Delete(keychainService(), name+"/"+kind); err != nil {
			return err
		}
	}
	return os.RemoveAll(Dir(name))
}
