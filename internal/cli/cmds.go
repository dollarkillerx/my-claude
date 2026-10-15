package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/desktop"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

type partFlags struct{ codeOnly, desktopOnly bool }

// platform adjusts the flags for platforms without Claude Desktop: the
// Desktop half is skipped, and asking for it explicitly is an error.
func (f *partFlags) platform() error {
	if desktop.Supported() {
		return nil
	}
	if f.desktopOnly {
		return i18n.E("err.desktop_unsupported")
	}
	f.codeOnly = true
	return nil
}

func (f *partFlags) register(c *cobra.Command) {
	c.Flags().BoolVar(&f.codeOnly, "code-only", false, i18n.T("flag.code_only"))
	c.Flags().BoolVar(&f.desktopOnly, "desktop-only", false, i18n.T("flag.desktop_only"))
	c.MarkFlagsMutuallyExclusive("code-only", "desktop-only")
}

// switchFlags are the flags of every command that switches accounts.
type switchFlags struct {
	partFlags
	noLaunch, force bool
}

func (f *switchFlags) register(c *cobra.Command) {
	f.partFlags.register(c)
	c.Flags().BoolVar(&f.noLaunch, "no-launch", false, i18n.T("flag.no_launch"))
	c.Flags().BoolVar(&f.force, "force", false, i18n.T("flag.force_use"))
}

func loadOrNew(name string) (*store.Profile, error) {
	p, err := store.Load(name)
	if errors.Is(err, store.ErrNoProfile) {
		return &store.Profile{Name: name}, nil
	}
	return p, err
}

// claimName returns the profile to save account uuid under name: the profile
// of that name, a new one, or, when the account is already saved under
// another name (for example an automatic one), that profile renamed to name.
func claimName(name, uuid string) (*store.Profile, error) {
	other, err := store.FindByAccount(uuid)
	if err != nil {
		return nil, err
	}
	if other == nil || other.Name == name {
		return loadOrNew(name)
	}
	if existing, err := store.Load(name); err == nil {
		return nil, i18n.E("err.name_taken", name, emailOrNote(existing))
	} else if !errors.Is(err, store.ErrNoProfile) {
		return nil, err
	}
	if err := store.Rename(other.Name, name); err != nil {
		return nil, err
	}
	info(i18n.T("info.renamed", other.Name, name))
	return store.Load(name)
}

// validateNewOrExisting accepts the name of an existing profile, or a name
// that is valid for a new one (not reserved).
func validateNewOrExisting(name string) error {
	if _, err := store.Load(name); err == nil {
		return nil
	}
	return store.ValidateNewName(name)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// emailOrNote is a profile's email, or a note for Desktop-only profiles,
// whose email Desktop does not expose.
func emailOrNote(p *store.Profile) string {
	if p.Email != "" {
		return p.Email
	}
	if p.HasDesktop {
		return i18n.T("list.desktop_only")
	}
	return "-"
}

// saveCode saves the live Claude Code login: under name if given, otherwise
// into the profile that already holds it or a new automatically named one.
// With renameExisting unset, an account already saved under another name
// stays there (the name is ignored with a notice) instead of being renamed.
func saveCode(name string, renameExisting bool) (*store.Profile, error) {
	st, err := identifyCode() // also repairs a stale oauthAccount
	if err != nil {
		return nil, err
	}
	if !st.loggedIn || st.account == nil {
		return nil, errCodeLoggedOut
	}
	var p *store.Profile
	switch {
	case name != "" && st.owner != nil && st.owner.Name != name && !renameExisting:
		info(i18n.T("info.already_saved_as", st.account.EmailAddress, st.owner.Name))
		p = st.owner
	case name != "":
		if p, err = claimName(name, st.account.AccountUUID); err != nil {
			return nil, err
		}
	case st.owner != nil:
		p = st.owner
	default:
		if name, err = autoName(st.account.EmailAddress, st.account.AccountUUID); err != nil {
			return nil, err
		}
		p = &store.Profile{Name: name}
	}
	if err := captureCode(p); err != nil {
		return nil, err
	}
	info(i18n.T("info.saved_code", p.Email, p.Name))
	return p, nil
}

// saveDesktop saves the live Desktop login. It goes into the profile that
// already holds that account, else into name (when given and the account
// fits), else into the profile just saved for Claude Code if it is the same
// account, else a new automatically named profile. Desktop is quit and
// reopened so its files are copied consistently; an account that is already
// saved is skipped unless always is set, to avoid needless restarts.
func saveDesktop(name string, codeProfile *store.Profile, always bool) (bool, error) {
	snap, err := desktop.Read()
	if errors.Is(err, desktop.ErrNotLoggedIn) && !always {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if snap.AccountUUID == "" {
		return false, i18n.E("err.desktop_unknown_account")
	}
	existing, err := store.FindByAccount(snap.AccountUUID)
	if err != nil {
		return false, err
	}
	if existing != nil && existing.HasDesktop && !always {
		return true, nil // already saved; refreshed on the next switch
	}
	var p *store.Profile
	switch {
	case name != "" && (codeProfile == nil || codeProfile.Name != name):
		if p, err = claimName(name, snap.AccountUUID); err != nil {
			return false, err
		}
	case existing != nil:
		p = existing
	case codeProfile != nil && codeProfile.AccountUUID == snap.AccountUUID:
		p = codeProfile
	default:
		auto, err := autoName("", snap.AccountUUID)
		if err != nil {
			return false, err
		}
		p = &store.Profile{Name: auto}
	}
	if desktop.Running() {
		info(i18n.T("info.desktop_restart_to_save"))
	}
	err = withDesktopStopped(false, func() error {
		if err := captureDesktop(p); err != nil {
			return err
		}
		info(i18n.T("info.saved_desktop", p.Name))
		return nil
	})
	return err == nil, err
}

func addCmd() *cobra.Command {
	var parts partFlags
	c := &cobra.Command{
		Use:     "add [name]",
		Aliases: []string{"save"},
		Short:   i18n.T("cmd.add.short"),
		Long:    i18n.T("cmd.add.long"),
		Example: "  cswitch add\n  cswitch add work",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := parts.platform(); err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
				if err := validateNewOrExisting(name); err != nil {
					return err
				}
			}
			var codeProfile *store.Profile
			if !parts.desktopOnly {
				p, err := saveCode(name, true)
				if err != nil && (parts.codeOnly || !errors.Is(err, errCodeLoggedOut)) {
					return err
				}
				codeProfile = p
			}
			if parts.codeOnly {
				return nil
			}
			deskName := name
			if codeProfile != nil && codeProfile.Name == name {
				deskName = "" // the name went to the Code account
			}
			saved, err := saveDesktop(deskName, codeProfile, parts.desktopOnly)
			if err != nil {
				return err
			}
			if codeProfile == nil && !saved {
				return i18n.E("err.nothing_to_save")
			}
			return nil
		},
	}
	parts.register(c)
	return c
}

type codeLoggedOut struct{}

func (codeLoggedOut) Error() string {
	if !desktop.Supported() {
		return i18n.T("err.code_not_logged_in_add_nodesktop")
	}
	return i18n.T("err.code_not_logged_in_add")
}

// errCodeLoggedOut is saveCode's "not logged in" error, so `add` can still
// save a Desktop login when Claude Code is logged out.
var errCodeLoggedOut error = codeLoggedOut{}

func loginCmd() *cobra.Command {
	var withDesktop, desktopOnly bool
	c := &cobra.Command{
		Use:     "login [name]",
		Short:   i18n.T("cmd.login.short"),
		Long:    i18n.T("cmd.login.long"),
		Example: "  cswitch login\n  cswitch login --desktop\n  cswitch login --desktop-only",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (withDesktop || desktopOnly) && !desktop.Supported() {
				return i18n.E("err.desktop_unsupported")
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
				if err := validateNewOrExisting(name); err != nil {
					return err
				}
			}
			var codeProfile *store.Profile
			if !desktopOnly {
				// Keep the current login: it is saved (automatically named if new).
				if _, err := saveBackCode(); err != nil {
					return err
				}
				login := exec.Command("claude", "auth", "login")
				login.Stdin, login.Stdout, login.Stderr = os.Stdin, os.Stdout, os.Stderr
				if err := login.Run(); err != nil {
					return i18n.E("err.claude_login", err)
				}
				resetLiveCache()
				p, err := saveCode(name, false)
				if err != nil {
					return err
				}
				codeProfile = p
			}
			if !withDesktop && !desktopOnly {
				return nil
			}

			if err := checkDesktopSavable(false); err != nil {
				return err
			}
			err := withDesktopStopped(true, func() error {
				if _, err := saveBackDesktop(false); err != nil {
					return err
				}
				if err := desktop.Restore(nil, store.BackupDir()); err != nil {
					return err
				}
				return desktop.RestoreState("", store.BackupDir())
			})
			if err != nil {
				return err
			}
			if codeProfile != nil {
				fmt.Fprint(os.Stderr, i18n.T("prompt.desktop_login_as", codeProfile.Email))
			} else {
				fmt.Fprint(os.Stderr, i18n.T("prompt.desktop_login"))
			}
			bufio.NewReader(os.Stdin).ReadString('\n')
			deskName := name
			if codeProfile != nil {
				deskName = "" // same account goes into the same profile by UUID
			}
			_, err = saveDesktop(deskName, codeProfile, true)
			return err
		},
	}
	c.Flags().BoolVar(&withDesktop, "desktop", false, i18n.T("flag.login_desktop"))
	c.Flags().BoolVar(&desktopOnly, "desktop-only", false, i18n.T("flag.login_desktop_only"))
	c.MarkFlagsMutuallyExclusive("desktop", "desktop-only")
	return c
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   i18n.T("cmd.list.short"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ps, err := store.List()
			if err != nil {
				return err
			}
			if len(ps) == 0 {
				info(i18n.T("info.no_profiles"))
				return nil
			}
			codeUUID, deskUUID := activeAccounts()
			rows := [][]string{splitHeader(i18n.T("list.header"))}
			for _, p := range ps {
				rows = append(rows, []string{p.Name, emailOrNote(p), orDash(p.SubscriptionType),
					slotMark(p.HasCode, p.AccountUUID == codeUUID),
					slotMark(p.HasDesktop, desktop.Supported() && p.AccountUUID == deskUUID)})
			}
			printTable(os.Stdout, rows)
			return nil
		},
	}
}

// activeAccounts returns the account UUIDs Claude Code and Desktop use. The
// Claude Code one is the account its token belongs to, not ~/.claude.json.
func activeAccounts() (code, desk string) {
	if st, err := identifyCode(); err == nil && st.account != nil {
		code = st.account.AccountUUID
	}
	return code, desktop.CurrentAccount()
}

// slotMark renders one app column: "* in use", "✓" saved, "-" not saved.
// (● and ○ are ambiguous-width and misalign columns in CJK terminals.)
func slotMark(saved, active bool) string {
	switch {
	case active:
		return "* " + i18n.T("list.in_use")
	case saved:
		return "✓"
	}
	return "-"
}

func currentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: i18n.T("cmd.current.short"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			describe := func(loggedIn bool, uuid, email string) string {
				if !loggedIn {
					return i18n.T("current.not_logged_in")
				}
				p, _ := store.FindByAccount(uuid)
				switch {
				case p != nil && uuid != "":
					return fmt.Sprintf("%s (%s)", p.Name, emailOrNote(p))
				case email != "":
					return i18n.T("current.not_saved", email)
				default:
					return i18n.T("current.not_saved", orDash(uuid))
				}
			}
			codeIn, code, email := false, "", ""
			st, err := identifyCode()
			if err != nil {
				return err
			}
			if st.loggedIn {
				codeIn = true
				if st.account != nil {
					code, email = st.account.AccountUUID, st.account.EmailAddress
				}
			}
			fmt.Printf("%-16s%s\n", "Claude Code:", describe(codeIn, code, email))
			if desktop.Supported() {
				snap, derr := desktop.Read()
				deskIn, desk := derr == nil, ""
				if deskIn {
					desk = snap.AccountUUID
				}
				fmt.Printf("%-16s%s\n", "Claude Desktop:", describe(deskIn, desk, ""))
			}
			return nil
		},
	}
}

func useCmd() *cobra.Command {
	var sw switchFlags
	c := &cobra.Command{
		Use:     "use <name>",
		Aliases: []string{"switch"},
		Short:   i18n.T("cmd.use.short"),
		Long:    i18n.T("cmd.use.long"),
		Example: "  cswitch use work\n  cswitch work\n  cswitch use personal --code-only",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return switchByName(cmd, args[0], sw)
		},
	}
	sw.register(c)
	return c
}

// switchByName switches to the named profile, or explains what exists when
// there is no such profile (including a likely mistyped command).
func switchByName(cmd *cobra.Command, name string, sw switchFlags) error {
	p, err := store.Load(name)
	if errors.Is(err, store.ErrNoProfile) || store.ValidateName(name) != nil {
		var names []string
		if ps, _ := store.List(); len(ps) > 0 {
			for _, p := range ps {
				names = append(names, p.Name)
			}
		}
		msg := i18n.T("err.no_such_account", name)
		if len(names) > 0 {
			msg += "\n" + i18n.T("hint.accounts", strings.Join(names, ", "))
		}
		if s := cmd.Root().SuggestionsFor(name); len(s) > 0 {
			msg += "\n" + i18n.T("hint.did_you_mean", s[0])
		}
		return errors.New(msg)
	}
	if err != nil {
		return err
	}
	return switchTo(p, sw)
}

// switchTo switches Claude Code and/or Desktop to p.
func switchTo(p *store.Profile, sw switchFlags) error {
	if err := sw.platform(); err != nil {
		return err
	}
	doCode, doDesktop := !sw.desktopOnly, !sw.codeOnly
	if doCode && !p.HasCode {
		if sw.codeOnly {
			return i18n.E("err.no_secret_code", p.Name)
		}
		info(i18n.T("info.skip_code", p.Name))
		doCode = false
	}
	if doDesktop && !p.HasDesktop {
		if sw.desktopOnly {
			return i18n.E("err.no_secret_desktop", p.Name)
		}
		if desktop.Supported() {
			info(i18n.T("info.skip_desktop", p.Name))
		}
		doDesktop = false
	}

	// Run every check before changing anything, so a refusal never leaves one
	// app switched and the other not. Unsaved current logins are saved here.
	var codeCur *store.Profile
	needCode := false
	if doCode {
		var err error
		if codeCur, err = codeOwner(); err != nil {
			return err
		}
		needCode = codeCur == nil || codeCur.Name != p.Name
	}
	needDesktop := doDesktop && desktop.CurrentAccount() != p.AccountUUID
	if needDesktop {
		if err := checkDesktopSavable(sw.force); err != nil {
			return err
		}
	}

	// Desktop goes first: it is the step that can fail for reasons outside our
	// control (the app refusing to quit), and it rolls itself back. The Code
	// switch is two local writes.
	if needDesktop {
		err := withDesktopStopped(!sw.noLaunch, func() error {
			if _, err := saveBackDesktop(sw.force); err != nil {
				return err
			}
			return applyDesktop(p)
		})
		if err != nil {
			return err
		}
		info(i18n.T("info.switched_desktop", p.Name))
	} else if doDesktop {
		info(i18n.T("info.already_desktop", p.Name))
	}

	if doCode {
		if codeCur != nil {
			if err := captureCode(codeCur); err != nil {
				return err
			}
		}
		if !needCode {
			info(i18n.T("info.already_code", p.Name))
			return nil
		}
		if err := applyCode(p); err != nil {
			if needDesktop {
				return i18n.E("err.code_failed_after_desktop", p.Name, err)
			}
			return err
		}
		info(i18n.T("info.switched_code", p.Name, p.Email))
		if claudecode.Running() {
			warn(i18n.T("warn.claude_running"))
		}
	}
	return nil
}

func renameCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rename <old> <new>",
		Aliases: []string{"mv"},
		Short:   i18n.T("cmd.rename.short"),
		Example: "  cswitch rename desktop-1a2b3c4d work",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.Rename(args[0], args[1]); err != nil {
				return err
			}
			info(i18n.T("info.renamed", args[0], args[1]))
			return nil
		},
	}
}

func removeCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   i18n.T("cmd.remove.short"),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(args[0])
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprint(os.Stderr, i18n.T("prompt.remove", p.Name, emailOrNote(p)))
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
					return i18n.E("err.aborted")
				}
			}
			if err := store.Remove(p.Name); err != nil {
				return err
			}
			info(i18n.T("info.removed", p.Name))
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("flag.yes"))
	return c
}

func langCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "lang [en|zh|ja]",
		Aliases: []string{"language"},
		Short:   i18n.T("cmd.lang.short"),
		Example: "  cswitch lang zh\n  cswitch lang auto",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := store.LoadSettings()
			if len(args) == 0 {
				cur := s.Lang
				if cur == "" {
					cur = "auto"
				}
				fmt.Println(i18n.T("info.lang_current", cur, string(i18n.Current())))
				return nil
			}
			v := strings.ToLower(args[0])
			if v == "auto" || v == "system" {
				s.Lang = ""
			} else if l, ok := i18n.Parse(v); ok {
				s.Lang = string(l)
			} else {
				return i18n.E("err.bad_lang", args[0])
			}
			if err := store.SaveSettings(s); err != nil {
				return err
			}
			if s.Lang != "" {
				i18n.SetLang(s.Lang)
				info(i18n.T("info.lang_saved", s.Lang))
			} else {
				info(i18n.T("info.lang_auto"))
			}
			if env := os.Getenv("CSWITCH_LANG"); env != "" {
				warn(i18n.T("warn.lang_env_overrides", env))
			}
			return nil
		},
	}
}

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: i18n.T("cmd.sync.short"),
		Long:  i18n.T("cmd.sync.long"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !desktop.Supported() {
				return i18n.E("err.desktop_unsupported")
			}
			if desktop.Running() {
				// withDesktopStopped syncs after fn and relaunches Desktop.
				return withDesktopStopped(false, func() error { return nil })
			}
			if err := runSync(); err != nil {
				return err
			}
			info(i18n.T("info.in_sync"))
			return nil
		},
	}
}
