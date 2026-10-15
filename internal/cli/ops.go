package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/desktop"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

const quitTimeout = 20 * time.Second

func desktopDir(p *store.Profile) string { return filepath.Join(store.Dir(p.Name), "desktop") }

func syncStatePath() string { return filepath.Join(store.Home(), "sync-state.json") }

func trashDir() string { return filepath.Join(store.BackupDir(), "sessions-trash") }

func checkpointDir() string { return filepath.Join(store.Home(), "tmp") }

// codeState describes the live Claude Code login.
type codeState struct {
	loggedIn bool
	account  *claudecode.Account // who the token belongs to; nil only if unknown
	owner    *store.Profile      // profile holding this login, or nil if unsaved
}

// identifyCode works out which profile the live Claude Code login belongs to.
//
// The account is the one the server reports for the token (see readLiveCode),
// not ~/.claude.json, which Claude Desktop's embedded Claude Code rewrites with
// Desktop's account. A saved profile owns the login if it holds the same
// refresh token and is that account, or else if it is that account. When
// ~/.claude.json names someone else, it is put right so Claude Code's own
// display matches its login.
func identifyCode() (*codeState, error) {
	lc, err := readLiveCode()
	if errors.Is(err, claudecode.ErrNotLoggedIn) {
		return &codeState{}, nil
	}
	if err != nil {
		return nil, err
	}
	st := &codeState{loggedIn: true, account: lc.account}

	ps, err := store.List()
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if !p.HasCode || p.AccountUUID != lc.account.AccountUUID {
			continue
		}
		raw, err := store.Secret(p.Name, "code")
		if err != nil {
			continue
		}
		if saved, err := claudecode.ParseCredentials(raw); err == nil && saved.OAuth.RefreshToken == lc.creds.OAuth.RefreshToken {
			st.owner = p
			break
		}
	}
	if st.owner == nil {
		if st.owner, err = store.FindByAccount(lc.account.AccountUUID); err != nil {
			return nil, err
		}
	}
	if lc.verified {
		if _, fileAcct, err := claudecode.ReadAccount(); err != nil || fileAcct.AccountUUID != lc.account.AccountUUID {
			if err := claudecode.WriteAccount(lc.oauthAccount, store.BackupDir()); err != nil {
				return nil, err
			}
		}
	}
	return st, nil
}

func (s *codeState) email() string {
	if s.account != nil {
		return s.account.EmailAddress
	}
	return "?"
}

// autoName picks a name for an account no profile holds yet: the local part
// of its email ("alex" for alex@example.com), or "desktop-<id>" when
// only a Desktop login with no email is known. A number is appended if the
// name is taken. Users can rename it later with `cswitch rename`.
func autoName(email, uuid string) (string, error) {
	base := ""
	if i := strings.IndexByte(email, '@'); i > 0 {
		base = sanitizeName(email[:i])
	}
	if base == "" {
		id := strings.ReplaceAll(uuid, "-", "")
		if len(id) > 8 {
			id = id[:8]
		}
		base = "desktop-" + id
	}
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		if store.ValidateNewName(name) != nil {
			continue // reserved (a command name) or otherwise unusable
		}
		if _, err := store.Load(name); errors.Is(err, store.ErrNoProfile) {
			return name, nil
		} else if err != nil {
			return "", err
		}
	}
}

// sanitizeName turns an email local part into a valid profile name.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == '+':
			b.WriteRune('-')
		}
	}
	name := strings.TrimLeft(b.String(), "._-")
	if len(name) > 48 {
		name = name[:48]
	}
	if store.ValidateName(name) != nil {
		return ""
	}
	return name
}

// codeOwner returns the profile holding the live Claude Code login. A login
// no profile holds yet is saved under an automatic name first, so switching
// away never loses it and never needs a separate `add`.
func codeOwner() (*store.Profile, error) {
	st, err := identifyCode()
	if err != nil {
		return nil, err
	}
	if !st.loggedIn || st.owner != nil {
		return st.owner, nil
	}
	if st.account == nil {
		return nil, i18n.E("err.code_no_oauth_account")
	}
	name, err := autoName(st.account.EmailAddress, st.account.AccountUUID)
	if err != nil {
		return nil, err
	}
	p := &store.Profile{Name: name}
	if err := captureCode(p); err != nil {
		return nil, err
	}
	info(i18n.T("info.auto_saved_code", st.account.EmailAddress, name))
	return p, nil
}

// checkDesktopSavable fails only when Desktop holds a login that cannot be
// saved because its account is unknown; switching would then lose it.
func checkDesktopSavable(force bool) error {
	snap, err := desktop.Read()
	if errors.Is(err, desktop.ErrNotLoggedIn) {
		return nil
	}
	if err != nil {
		return err
	}
	if snap.AccountUUID == "" && !force {
		return i18n.E("err.desktop_unknown_account_switch")
	}
	return nil
}

// captureCode saves the live Claude Code login into p, labeled with the
// account the server says its token belongs to.
func captureCode(p *store.Profile) error {
	lc, err := readLiveCode()
	if err != nil {
		return err
	}
	acct := lc.account
	if p.AccountUUID != "" && p.AccountUUID != acct.AccountUUID {
		return i18n.E("err.profile_other_account", p.Name, emailOrNote(p), acct.EmailAddress)
	}
	if err := store.SetSecret(p.Name, "code", lc.creds.Raw); err != nil {
		return err
	}
	p.AccountUUID = acct.AccountUUID
	p.Email = acct.EmailAddress
	p.OrganizationName = acct.OrganizationName
	p.OrganizationUUID = acct.OrganizationUUID
	p.SubscriptionType = lc.creds.OAuth.SubscriptionType
	p.OAuthAccount = lc.oauthAccount
	p.HasCode = true
	return store.Save(p)
}

// captureDesktop saves the live Desktop login into p. Desktop should not be
// running, so the cookie database and web storage are copied consistently.
func captureDesktop(p *store.Profile) error {
	snap, err := desktop.Read()
	if err != nil {
		return err
	}
	if snap.AccountUUID == "" {
		return i18n.E("err.desktop_unknown_account")
	}
	if p.AccountUUID != "" && p.AccountUUID != snap.AccountUUID {
		return i18n.E("err.desktop_other_account", snap.AccountUUID, p.Name)
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	if err := store.SetSecret(p.Name, "desktop", string(data)); err != nil {
		return err
	}
	if err := desktop.SaveState(desktopDir(p)); err != nil {
		return err
	}
	p.AccountUUID = snap.AccountUUID
	p.HasDesktop = true
	return store.Save(p)
}

// saveBackCode stores the live Claude Code login into the profile that holds
// it (creating one if needed), so a refresh token Claude Code rotated since
// the last save is not lost.
func saveBackCode() (*store.Profile, error) {
	p, err := codeOwner()
	if err != nil || p == nil {
		return nil, err
	}
	return p, captureCode(p)
}

// saveBackDesktop is saveBackCode for Desktop. Desktop must not be running.
// With force, a login whose account is unknown is skipped instead of failing.
func saveBackDesktop(force bool) (*store.Profile, error) {
	snap, err := desktop.Read()
	if errors.Is(err, desktop.ErrNotLoggedIn) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if snap.AccountUUID == "" {
		if force {
			warn(i18n.T("warn.desktop_unknown_lost"))
			return nil, nil
		}
		return nil, i18n.E("err.desktop_unknown_account_switch")
	}
	p, err := store.FindByAccount(snap.AccountUUID)
	if err != nil {
		return nil, err
	}
	created := false
	if p == nil {
		name, err := autoName("", snap.AccountUUID)
		if err != nil {
			return nil, err
		}
		p, created = &store.Profile{Name: name}, true
	}
	if err := captureDesktop(p); err != nil {
		return nil, err
	}
	if created {
		info(i18n.T("info.auto_saved_desktop", p.Name))
	}
	return p, nil
}

// applyCode makes p the live Claude Code login. If updating ~/.claude.json
// fails after the keychain was written, the keychain is put back as it was:
// the previous credentials, or no item if Claude Code was logged out.
func applyCode(p *store.Profile) error {
	raw, err := store.Secret(p.Name, "code")
	if err != nil {
		return err
	}
	creds, err := claudecode.ParseCredentials(raw)
	if err != nil {
		return err
	}
	if len(p.OAuthAccount) == 0 {
		return i18n.E("err.no_oauth_account", p.Name)
	}
	prev, prevErr := claudecode.ReadCredentials()
	if prevErr != nil && !errors.Is(prevErr, claudecode.ErrNotLoggedIn) {
		// Without the previous credentials a failure below could not be undone.
		return prevErr
	}
	if err := claudecode.WriteCredentials(creds); err != nil {
		return err
	}
	resetLiveCache()
	if err := claudecode.WriteAccount(p.OAuthAccount, store.BackupDir()); err != nil {
		var rerr error
		if prev != nil {
			rerr = claudecode.WriteCredentials(prev)
		} else {
			rerr = claudecode.DeleteCredentials() // there was no login before
		}
		if rerr != nil {
			return i18n.E("err.rollback_failed", err, rerr)
		}
		return err
	}
	return nil
}

// applyDesktop makes p the live Desktop login. Desktop must not be running.
func applyDesktop(p *store.Profile) error {
	raw, err := store.Secret(p.Name, "desktop")
	if err != nil {
		return err
	}
	var snap desktop.Snapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return i18n.E("err.bad_desktop_snapshot", p.Name, err)
	}
	if err := desktop.Restore(&snap, store.BackupDir()); err != nil {
		return err
	}
	return desktop.RestoreState(desktopDir(p), store.BackupDir())
}

// withDesktopStopped quits Desktop, runs fn, syncs sessions, and relaunches
// Desktop if it was running or launch is set. Desktop's login state is
// checkpointed first; if fn fails, the checkpoint is restored so Desktop never
// comes back up with one account's tokens and another account's cookies.
func withDesktopStopped(launch bool, fn func() error) error {
	wasRunning := desktop.Running()
	if wasRunning {
		info(i18n.T("info.desktop_quitting"))
		if err := desktop.Quit(quitTimeout); err != nil {
			return err
		}
	}
	relaunch := func() {
		if wasRunning {
			_ = desktop.Launch()
		}
	}

	// From the checkpoint until the rollback decision, an interrupt would
	// leave Desktop half switched and the checkpoint (a copy of its tokens and
	// cookies) on disk, so hold off Ctrl-C and termination until it is done.
	stopSignals := deferSignals()
	defer stopSignals()

	cp, err := desktop.NewCheckpoint(checkpointDir())
	if err != nil {
		relaunch()
		return err
	}
	defer cp.Discard()

	if err := fn(); err != nil {
		if rerr := cp.Rollback(store.BackupDir()); rerr != nil {
			err = i18n.E("err.rollback_failed", err, rerr)
		} else {
			info(i18n.T("info.desktop_rolled_back"))
		}
		relaunch()
		return err
	}
	if err := runSync(); err != nil {
		warn(i18n.T("warn.sync_failed", err))
	}
	if wasRunning || launch {
		info(i18n.T("info.desktop_opening"))
		return desktop.Launch()
	}
	return nil
}

// deferSignals delays SIGINT/SIGTERM/SIGHUP until the returned function is
// called; a signal received in between then terminates the process normally.
func deferSignals() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return func() {
		signal.Stop(ch)
		select {
		case sig := <-ch:
			info(i18n.T("info.interrupted"))
			if s, ok := sig.(syscall.Signal); ok {
				os.Exit(128 + int(s))
			}
			os.Exit(1)
		default:
		}
	}
}

func runSync() error {
	res, err := desktop.Sync(syncStatePath(), trashDir())
	if err != nil {
		return err
	}
	if res.Copied > 0 || res.Removed > 0 {
		info(i18n.T("info.synced", res.Scopes, res.Copied, res.Removed))
	}
	return nil
}
