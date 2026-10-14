package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
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
	account  *claudecode.Account // nil if ~/.claude.json has no usable oauthAccount
	owner    *store.Profile      // profile holding this login, or nil if unsaved
}

// identifyCode works out which profile the live Claude Code login belongs to.
//
// The keychain token is matched against saved profiles first, because a
// claude process that was running during a switch can write its stale
// oauthAccount back into ~/.claude.json; the keychain is authoritative. When
// the two disagree, ~/.claude.json is repaired. If Claude Code has rotated the
// token since it was saved, the match falls back to oauthAccount.
func identifyCode() (*codeState, error) {
	creds, err := claudecode.ReadCredentials()
	if errors.Is(err, claudecode.ErrNotLoggedIn) {
		return &codeState{}, nil
	}
	if err != nil {
		return nil, err
	}
	st := &codeState{loggedIn: true}
	_, acct, acctErr := claudecode.ReadAccount()
	if acctErr == nil {
		st.account = acct
	}

	ps, err := store.List()
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if !p.HasCode {
			continue
		}
		raw, err := store.Secret(p.Name, "code")
		if err != nil {
			continue
		}
		saved, err := claudecode.ParseCredentials(raw)
		if err != nil || saved.OAuth.RefreshToken != creds.OAuth.RefreshToken {
			continue
		}
		st.owner = p
		if (st.account == nil || st.account.AccountUUID != p.AccountUUID) && len(p.OAuthAccount) > 0 {
			warn(i18n.T("warn.code_account_repaired", p.Name))
			if err := claudecode.WriteAccount(p.OAuthAccount, store.BackupDir()); err != nil {
				return nil, err
			}
			if _, st.account, err = claudecode.ReadAccount(); err != nil {
				return nil, err
			}
		}
		return st, nil
	}

	if st.account != nil {
		if st.owner, err = store.FindByAccount(st.account.AccountUUID); err != nil {
			return nil, err
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

// codeOwner returns the profile owning the live Claude Code login. If Claude
// Code is logged in with an account no profile holds, switching away would
// lose that login, so it fails unless force is set.
func codeOwner(force bool) (*store.Profile, error) {
	st, err := identifyCode()
	if err != nil {
		return nil, err
	}
	if st.loggedIn && st.owner == nil {
		if !force {
			return nil, i18n.E("err.code_unsaved", st.email())
		}
		warn(i18n.T("warn.code_unsaved_lost", st.email()))
	}
	return st.owner, nil
}

// desktopOwner is codeOwner for Claude Desktop.
func desktopOwner(force bool) (*store.Profile, error) {
	snap, err := desktop.Read()
	if errors.Is(err, desktop.ErrNotLoggedIn) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p *store.Profile
	if snap.AccountUUID != "" {
		if p, err = store.FindByAccount(snap.AccountUUID); err != nil {
			return nil, err
		}
	}
	if p == nil {
		id := snap.AccountUUID
		if id == "" {
			id = "?"
		}
		if !force {
			return nil, i18n.E("err.desktop_unsaved", id)
		}
		warn(i18n.T("warn.desktop_unsaved_lost", id))
	}
	return p, nil
}

// captureCode saves the live Claude Code login into p.
func captureCode(p *store.Profile) error {
	creds, err := claudecode.ReadCredentials()
	if err != nil {
		return err
	}
	raw, acct, err := claudecode.ReadAccount()
	if err != nil {
		return err
	}
	if p.AccountUUID != "" && p.AccountUUID != acct.AccountUUID {
		return i18n.E("err.profile_other_account", p.Name, orDash(p.Email), acct.EmailAddress)
	}
	if err := store.SetSecret(p.Name, "code", creds.Raw); err != nil {
		return err
	}
	p.AccountUUID = acct.AccountUUID
	p.Email = acct.EmailAddress
	p.OrganizationName = acct.OrganizationName
	p.OrganizationUUID = acct.OrganizationUUID
	p.SubscriptionType = creds.OAuth.SubscriptionType
	p.OAuthAccount = raw
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

// saveBackCode stores the live Claude Code login into the profile that owns
// it, so a refresh token Claude Code rotated since the last save is not lost.
func saveBackCode(force bool) (*store.Profile, error) {
	p, err := codeOwner(force)
	if err != nil || p == nil {
		return nil, err
	}
	return p, captureCode(p)
}

// saveBackDesktop is saveBackCode for Desktop. Desktop must not be running.
func saveBackDesktop(force bool) (*store.Profile, error) {
	p, err := desktopOwner(force)
	if err != nil || p == nil {
		return nil, err
	}
	return p, captureDesktop(p)
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
