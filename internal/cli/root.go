// Package cli implements the cswitch commands.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/desktop"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

// Version is set at build time with -ldflags "-X github.com/dollarkillerx/my-claude/internal/cli.Version=v1.2.3".
var Version = "dev"

func info(msg string) { fmt.Fprintln(os.Stderr, msg) }
func warn(msg string) { fmt.Fprintln(os.Stderr, i18n.T("prefix.warning")+msg) }

// NewRoot builds the command tree. Help text is translated when the tree is
// built, so the language must be set before calling it.
func NewRoot() *cobra.Command {
	var lang string
	root := &cobra.Command{
		Use:           "cswitch",
		Short:         i18n.T("cmd.root.short"),
		Long:          i18n.T("cmd.root.long"),
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Parsed early by Execute; registered here so it shows up in help and is accepted.
	root.PersistentFlags().StringVar(&lang, "lang", "", i18n.T("flag.lang"))
	root.AddCommand(addCmd(), loginCmd(), listCmd(), currentCmd(), useCmd(), usageCmd(), removeCmd(), syncCmd())
	localizeHelp(root)
	return root
}

// Execute runs the CLI.
func Execute() {
	if l := langFromArgs(os.Args[1:]); l != "" && !i18n.SetLang(l) {
		fmt.Fprintln(os.Stderr, i18n.T("err.bad_lang", l))
		os.Exit(2)
	}
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("prefix.error")+err.Error())
		os.Exit(1)
	}
}

// langFromArgs finds --lang before cobra parses flags, because help text is
// translated while the command tree is built.
func langFromArgs(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		if v, ok := strings.CutPrefix(a, "--lang="); ok {
			return v
		}
		if a == "--lang" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// localizeHelp translates cobra's built-in help headings and flags.
func localizeHelp(root *cobra.Command) {
	tmpl := root.UsageTemplate()
	for _, h := range []struct{ en, key string }{
		{"Usage:", "help.usage"},
		{"Aliases:", "help.aliases"},
		{"Examples:", "help.examples"},
		{"Available Commands:", "help.commands"},
		{"Additional Commands:", "help.additional_commands"},
		{"Global Flags:", "help.global_flags"},
		{"Flags:", "help.flags"},
		{"Additional help topics:", "help.topics"},
		{`Use "{{.CommandPath}} [command] --help" for more information about a command.`, "help.more"},
	} {
		tmpl = strings.ReplaceAll(tmpl, h.en, i18n.T(h.key))
	}
	root.SetUsageTemplate(tmpl)
	root.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: i18n.T("cmd.help.short"),
		Run: func(c *cobra.Command, args []string) {
			cmd, _, err := c.Root().Find(args)
			if cmd == nil || err != nil {
				cmd = c.Root()
			}
			cmd.Help()
		},
	})
	root.CompletionOptions.HiddenDefaultCmd = true
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		if f := c.Flags().Lookup("help"); f != nil {
			f.Usage = i18n.T("flag.help", c.Name())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	root.InitDefaultVersionFlag()
	if f := root.Flags().Lookup("version"); f != nil {
		f.Usage = i18n.T("flag.version")
	}
}

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

func loadOrNew(name string) (*store.Profile, error) {
	p, err := store.Load(name)
	if errors.Is(err, store.ErrNoProfile) {
		return &store.Profile{Name: name}, nil
	}
	return p, err
}

// checkAccountFree fails if uuid is already saved under a different profile name.
func checkAccountFree(name, uuid string) error {
	other, err := store.FindByAccount(uuid)
	if err != nil {
		return err
	}
	if other != nil && other.Name != name {
		return i18n.E("err.account_taken", other.Name)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// addCode saves the live Claude Code login as profile p.
func addCode(p *store.Profile) error {
	st, err := identifyCode() // also repairs a stale oauthAccount
	if err != nil {
		return err
	}
	if !st.loggedIn || st.account == nil {
		return i18n.E("err.code_not_logged_in_add")
	}
	if err := checkAccountFree(p.Name, st.account.AccountUUID); err != nil {
		return err
	}
	if err := captureCode(p); err != nil {
		return err
	}
	info(i18n.T("info.saved_code", p.Email, p.Name))
	return nil
}

// addDesktop saves the live Desktop login as profile p. Desktop is quit first
// (and reopened) so its cookie database and storage are copied consistently.
// With required unset, a Desktop logged in as another account is only noted.
func addDesktop(p *store.Profile, required bool) error {
	snap, err := desktop.Read()
	if errors.Is(err, desktop.ErrNotLoggedIn) && !required {
		return nil
	}
	if err != nil {
		return err
	}
	if snap.AccountUUID == "" {
		return i18n.E("err.desktop_unknown_account")
	}
	if p.AccountUUID != "" && snap.AccountUUID != p.AccountUUID {
		if required {
			return i18n.E("err.desktop_other_account", snap.AccountUUID, p.Name)
		}
		info(i18n.T("info.desktop_other_account"))
		return nil
	}
	if err := checkAccountFree(p.Name, snap.AccountUUID); err != nil {
		return err
	}
	if desktop.Running() {
		info(i18n.T("info.desktop_restart_to_save"))
	}
	return withDesktopStopped(false, func() error {
		if err := captureDesktop(p); err != nil {
			return err
		}
		info(i18n.T("info.saved_desktop", p.Name))
		return nil
	})
}

func addCmd() *cobra.Command {
	var parts partFlags
	c := &cobra.Command{
		Use:     "add <name>",
		Short:   i18n.T("cmd.add.short"),
		Long:    i18n.T("cmd.add.long"),
		Example: "  cswitch add work\n  cswitch add personal --desktop-only",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := parts.platform(); err != nil {
				return err
			}
			p, err := loadOrNew(args[0])
			if err != nil {
				return err
			}
			if !parts.desktopOnly {
				if err := addCode(p); err != nil {
					return err
				}
			}
			if !parts.codeOnly {
				return addDesktop(p, parts.desktopOnly)
			}
			return nil
		},
	}
	parts.register(c)
	return c
}

func loginCmd() *cobra.Command {
	var parts partFlags
	var force bool
	c := &cobra.Command{
		Use:     "login <name>",
		Short:   i18n.T("cmd.login.short"),
		Long:    i18n.T("cmd.login.long"),
		Example: "  cswitch login personal\n  cswitch login personal --code-only",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := parts.platform(); err != nil {
				return err
			}
			if err := store.ValidateName(args[0]); err != nil {
				return err
			}
			p, err := loadOrNew(args[0])
			if err != nil {
				return err
			}
			// Check both sides before changing anything.
			if !parts.desktopOnly {
				if _, err := codeOwner(force); err != nil {
					return err
				}
			}
			if !parts.codeOnly {
				if _, err := desktopOwner(force); err != nil {
					return err
				}
			}

			if !parts.desktopOnly {
				if _, err := saveBackCode(force); err != nil {
					return err
				}
				login := exec.Command("claude", "auth", "login")
				login.Stdin, login.Stdout, login.Stderr = os.Stdin, os.Stdout, os.Stderr
				if err := login.Run(); err != nil {
					return i18n.E("err.claude_login", err)
				}
				if err := addCode(p); err != nil {
					return err
				}
			}
			if parts.codeOnly {
				return nil
			}

			err = withDesktopStopped(true, func() error {
				if _, err := saveBackDesktop(force); err != nil {
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
			if p.Email != "" {
				fmt.Fprint(os.Stderr, i18n.T("prompt.desktop_login_as", p.Email))
			} else {
				fmt.Fprint(os.Stderr, i18n.T("prompt.desktop_login"))
			}
			bufio.NewReader(os.Stdin).ReadString('\n')
			return addDesktop(p, true)
		},
	}
	parts.register(c)
	c.Flags().BoolVar(&force, "force", false, i18n.T("flag.force_login"))
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
			codeUUID := ""
			if _, a, err := claudecode.ReadAccount(); err == nil {
				codeUUID = a.AccountUUID
			}
			deskUUID := desktop.CurrentAccount()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("list.header"))
			for _, p := range ps {
				active := ""
				if p.AccountUUID == codeUUID {
					active += "C"
				}
				if p.AccountUUID == deskUUID {
					active += "D"
				}
				var saved []string
				if p.HasCode {
					saved = append(saved, "code")
				}
				if p.HasDesktop {
					saved = append(saved, "desktop")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", orDash(active), p.Name, orDash(p.Email), orDash(p.OrganizationName), orDash(p.SubscriptionType), strings.Join(saved, ","))
			}
			return w.Flush()
		},
	}
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
					return fmt.Sprintf("%s (%s)", p.Name, orDash(p.Email))
				case email != "":
					return i18n.T("current.not_saved", email)
				default:
					return i18n.T("current.not_saved", orDash(uuid))
				}
			}
			codeIn, code, email := false, "", ""
			if _, err := claudecode.ReadCredentials(); err == nil {
				codeIn = true
			}
			if _, a, err := claudecode.ReadAccount(); err == nil {
				code, email = a.AccountUUID, a.EmailAddress
			}
			snap, derr := desktop.Read()
			deskIn, desk := derr == nil, ""
			if deskIn {
				desk = snap.AccountUUID
			}
			fmt.Printf("%-16s%s\n", "Claude Code:", describe(codeIn, code, email))
			fmt.Printf("%-16s%s\n", "Claude Desktop:", describe(deskIn, desk, ""))
			return nil
		},
	}
}

func useCmd() *cobra.Command {
	var parts partFlags
	var force, noLaunch bool
	c := &cobra.Command{
		Use:     "use <name>",
		Aliases: []string{"switch"},
		Short:   i18n.T("cmd.use.short"),
		Long:    i18n.T("cmd.use.long"),
		Example: "  cswitch use work\n  cswitch use personal --code-only",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := parts.platform(); err != nil {
				return err
			}
			p, err := store.Load(args[0])
			if err != nil {
				return err
			}
			doCode, doDesktop := !parts.desktopOnly, !parts.codeOnly
			if doCode && !p.HasCode {
				if parts.codeOnly {
					return i18n.E("err.no_secret_code", p.Name)
				}
				info(i18n.T("info.skip_code", p.Name))
				doCode = false
			}
			if doDesktop && !p.HasDesktop {
				if parts.desktopOnly {
					return i18n.E("err.no_secret_desktop", p.Name)
				}
				info(i18n.T("info.skip_desktop", p.Name))
				doDesktop = false
			}

			// Run every check before changing anything, so a refusal never
			// leaves one app switched and the other not.
			var codeCur *store.Profile
			needCode := false
			if doCode {
				if codeCur, err = codeOwner(force); err != nil {
					return err
				}
				needCode = codeCur == nil || codeCur.Name != p.Name
			}
			needDesktop := doDesktop && desktop.CurrentAccount() != p.AccountUUID
			if needDesktop {
				if _, err := desktopOwner(force); err != nil {
					return err
				}
			}

			// Desktop goes first: it is the step that can fail for reasons
			// outside our control (the app refusing to quit), and it rolls
			// itself back. The Code switch is two local writes.
			if needDesktop {
				err := withDesktopStopped(!noLaunch, func() error {
					if _, err := saveBackDesktop(force); err != nil {
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
		},
	}
	parts.register(c)
	c.Flags().BoolVar(&force, "force", false, i18n.T("flag.force_use"))
	c.Flags().BoolVar(&noLaunch, "no-launch", false, i18n.T("flag.no_launch"))
	return c
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
				fmt.Fprint(os.Stderr, i18n.T("prompt.remove", p.Name, orDash(p.Email)))
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
