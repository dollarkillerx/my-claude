package cli

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

// verifyTimeout bounds the server lookup of who a token belongs to.
const verifyTimeout = 10 * time.Second

// liveCode is the live Claude Code login together with the account its token
// actually belongs to.
//
// ~/.claude.json cannot be trusted for that: Claude Desktop's embedded Claude
// Code shares the file and rewrites oauthAccount with Desktop's account, so it
// often names a different account than the CLI's keychain token. The server
// is asked instead; the file is only a fallback when offline.
type liveCode struct {
	creds        *claudecode.Credentials
	account      *claudecode.Account
	oauthAccount json.RawMessage // what to store and later restore as oauthAccount
	verified     bool            // account confirmed by the server
}

var (
	liveMu     sync.Mutex
	liveCached *liveCode
	offlineMsg sync.Once
)

// readLiveCode returns the live login, or claudecode.ErrNotLoggedIn. The
// result is cached per token for the rest of the run.
func readLiveCode() (*liveCode, error) {
	creds, err := claudecode.ReadCredentials()
	if err != nil {
		return nil, err
	}
	liveMu.Lock()
	defer liveMu.Unlock()
	if liveCached != nil && liveCached.creds.Raw == creds.Raw {
		return liveCached, nil
	}

	rawJSON, jsonAcct, jsonErr := claudecode.ReadAccount()
	lc := &liveCode{creds: creds}
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()
	prof, err := profileOf(ctx, creds, func(c *claudecode.Credentials) error {
		lc.creds = c
		return claudecode.WriteCredentials(c)
	})
	switch {
	case err == nil:
		lc.verified = true
		lc.account = &claudecode.Account{
			AccountUUID:      prof.AccountUUID,
			EmailAddress:     prof.Email,
			DisplayName:      prof.DisplayName,
			OrganizationUUID: prof.OrganizationUUID,
			OrganizationName: prof.OrganizationName,
			BillingType:      prof.BillingType,
		}
		if jsonErr == nil && jsonAcct.AccountUUID == prof.AccountUUID {
			lc.oauthAccount = rawJSON
		} else {
			lc.oauthAccount = oauthAccountFrom(prof)
		}
	case jsonErr == nil:
		offlineMsg.Do(func() { warn(i18n.T("warn.unverified_account", err)) })
		lc.account, lc.oauthAccount = jsonAcct, rawJSON
	default:
		return nil, err
	}
	liveCached = lc
	return lc, nil
}

// profileOf asks the server who creds belong to. An expired access token is
// refreshed first (and once more on 401); save persists refreshed credentials.
func profileOf(ctx context.Context, creds *claudecode.Credentials, save func(*claudecode.Credentials) error) (*usage.Profile, error) {
	refresh := func() error {
		t, err := usage.Refresh(ctx, creds.OAuth.RefreshToken)
		if err != nil {
			return err
		}
		if creds, err = creds.WithTokens(t.AccessToken, t.RefreshToken, t.ExpiresAt); err != nil {
			return err
		}
		return save(creds)
	}
	refreshed := false
	if creds.Expired(2 * time.Minute) {
		if err := refresh(); err != nil {
			return nil, err
		}
		refreshed = true
	}
	prof, err := usage.FetchProfile(ctx, creds.OAuth.AccessToken)
	if usage.IsUnauthorized(err) && !refreshed {
		if rerr := refresh(); rerr != nil {
			return nil, rerr
		}
		prof, err = usage.FetchProfile(ctx, creds.OAuth.AccessToken)
	}
	return prof, err
}

// oauthAccountFrom builds the oauthAccount entry Claude Code keeps in
// ~/.claude.json from the server's profile.
func oauthAccountFrom(p *usage.Profile) json.RawMessage {
	m := map[string]string{
		"accountUuid":               p.AccountUUID,
		"emailAddress":              p.Email,
		"displayName":               p.DisplayName,
		"organizationUuid":          p.OrganizationUUID,
		"organizationName":          p.OrganizationName,
		"organizationType":          p.OrganizationType,
		"organizationRateLimitTier": p.RateLimitTier,
		"billingType":               p.BillingType,
	}
	for k, v := range m {
		if v == "" {
			delete(m, k)
		}
	}
	data, _ := json.Marshal(m)
	return data
}

// resetLiveCache forgets the cached live login (after Claude Code's login
// changed under us, e.g. "claude auth login" or a switch).
func resetLiveCache() {
	liveMu.Lock()
	liveCached = nil
	liveMu.Unlock()
}

// fixMislabeled handles a saved Claude Code login that the server says
// belongs to another account than its profile (saved by an earlier version
// that trusted ~/.claude.json). The login is moved to the profile of the
// account it belongs to, or to a new profile, and removed from p. It returns
// the error to show for p.
func fixMislabeled(p *store.Profile, creds *claudecode.Credentials, owner *usage.Profile) error {
	target, err := store.FindByAccount(owner.AccountUUID)
	if err != nil {
		return err
	}
	if target == nil {
		name, err := autoName(owner.Email, owner.AccountUUID)
		if err != nil {
			return err
		}
		target = &store.Profile{Name: name}
	}
	if !target.HasCode {
		if err := store.SetSecret(target.Name, "code", creds.Raw); err != nil {
			return err
		}
		target.AccountUUID = owner.AccountUUID
		target.Email = owner.Email
		target.OrganizationName = owner.OrganizationName
		target.OrganizationUUID = owner.OrganizationUUID
		target.SubscriptionType = creds.OAuth.SubscriptionType
		target.OAuthAccount = oauthAccountFrom(owner)
		target.HasCode = true
		if err := store.Save(target); err != nil {
			return err
		}
	}
	if err := store.DeleteSecret(p.Name, "code"); err != nil {
		return err
	}
	p.HasCode = false
	p.SubscriptionType = ""
	if err := store.Save(p); err != nil {
		return err
	}
	return i18n.E("err.code_login_mislabeled", p.Name, owner.Email, target.Name)
}
