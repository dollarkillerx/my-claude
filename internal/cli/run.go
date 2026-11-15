package cli

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

// Session mode: `cswitch run <name>` starts Claude Code as <name> in this
// terminal only, by pointing Claude Code's own CLAUDE_CONFIG_DIR at a
// per-account directory. Every other terminal (and the global login) stays
// as it is, so several accounts can work at the same time.
//
// The session directory shares the default setup through symlinks to
// ~/.claude (settings, CLAUDE.md, skills, plugins, conversation history), so
// a conversation started under one account can be resumed under another. It
// has its own credentials (seeded from the profile) and its own global config
// (a copy of ~/.claude.json with the profile's account). When the last
// session of an account exits, the token it may have refreshed is saved back
// into the profile.

// sharedEntries are the parts of ~/.claude a session links to rather than
// keeping its own copy.
var sharedEntries = []string{
	"settings.json", "CLAUDE.md", "keybindings.json",
	"skills", "commands", "agents", "plugins", "output-styles", "hooks",
	"projects", "history.jsonl", "todos", "plans",
}

// sessionHome is the Claude Code home of p's session profile.
func sessionHome(p *store.Profile) claudecode.Home {
	return claudecode.Home{Dir: filepath.Join(store.Home(), "sessions", p.Name)}
}

func pidDir(h claudecode.Home) string { return filepath.Join(h.Dir, ".cswitch-pids") }

// livePids returns the pids of running sessions in h, removing stale entries.
func livePids(h claudecode.Home) []int {
	entries, _ := os.ReadDir(pidDir(h))
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if syscall.Kill(pid, 0) == nil {
			pids = append(pids, pid)
		} else {
			os.Remove(filepath.Join(pidDir(h), e.Name()))
		}
	}
	return pids
}

// sessionRunning reports whether a session of p is running.
func sessionRunning(p *store.Profile) bool { return len(livePids(sessionHome(p))) > 0 }

func runCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "run [name] [claude-args...]",
		Short:   i18n.T("cmd.run.short"),
		Long:    i18n.T("cmd.run.long"),
		Example: "  cswitch run work\n  cswitch run work --resume\n  cswitch run\n  cswitch run -- \"fix the tests\"",
		// claude's own flags pass through as they are; see parseRunArgs.
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, claudeArgs, help := parseRunArgs(args)
			if help {
				return cmd.Help()
			}
			if name == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				if dir, err := store.CleanDir(cwd); err == nil {
					if n, mapped := store.LoadMappings().Lookup(dir); n != "" {
						name = n
						info(i18n.T("info.run_mapped", mapped, n))
					}
				}
			}
			if name == "" {
				return execClaude(claudecode.Default, claudeArgs) // unmapped: the global login
			}
			p, err := store.Load(name)
			if err != nil {
				return err
			}
			return runSession(p, claudeArgs)
		},
	}
	return c
}

// parseRunArgs splits run's arguments into the account name and claude's
// arguments. cswitch does not parse them as flags, so `cswitch run work
// --resume` works without a "--": the first argument is the name unless it
// starts with "-", and one "--" right after the name (or first) is dropped.
// cswitch's own --help and --lang are recognized only before the name.
func parseRunArgs(args []string) (name string, rest []string, help bool) {
own:
	for len(args) > 0 {
		switch a := args[0]; {
		case a == "-h" || a == "--help":
			return "", nil, true
		case a == "--lang" && len(args) > 1:
			args = args[2:]
		case strings.HasPrefix(a, "--lang="):
			args = args[1:]
		default:
			break own
		}
	}
	if len(args) > 0 && args[0] == "--" {
		return "", args[1:], false
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
		if len(args) > 0 && args[0] == "--" {
			args = args[1:]
		}
	}
	return name, args, false
}

// execClaude runs claude in home and waits for it.
func execClaude(home claudecode.Home, args []string) error {
	cmd := exec.Command("claude", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if !home.IsDefault() {
		cmd.Env = append(cmd.Env, "CLAUDE_CONFIG_DIR="+home.Dir)
	}
	return passThrough(cmd.Run())
}

// passThrough exits with claude's exit status, or returns err when claude
// could not run. Ctrl-C belongs to claude meanwhile; see ignoreSignals.
func passThrough(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	return err
}

// ignoreSignals keeps cswitch alive while claude, which gets the same
// signals from the terminal, handles them. Call the returned func after.
func ignoreSignals() func() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for range sigs {
		}
	}()
	return func() { signal.Stop(sigs); close(sigs) }
}

// runSession prepares p's session home, runs claude in it, and saves the
// session's refreshed token back into the profile when the last one exits.
func runSession(p *store.Profile, args []string) error {
	home := sessionHome(p)
	pidFile := filepath.Join(pidDir(home), strconv.Itoa(os.Getpid()))
	global := false
	err := store.WithOpLock(func() error {
		// Checked under the lock, so an auto-switch cannot make p the global
		// login between this check and the pid file below.
		if liveProfileName() == p.Name {
			// Already the global login: a second copy of the same token in a
			// session would be rotated out from under one of them.
			global = true
			return nil
		}
		if err := prepareSession(p, home); err != nil {
			return err
		}
		// Registered before the lock is released: from now on, switching the
		// global login to p is refused until the session ends.
		return os.WriteFile(pidFile, nil, 0o600)
	})
	if err != nil {
		os.Remove(pidFile)
		return err
	}
	if global {
		return execClaude(claudecode.Default, args)
	}
	info(i18n.T("info.run_session", p.Name, emailOrNote(p)))

	cmd := exec.Command("claude", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+home.Dir)
	restore := ignoreSignals()
	runErr := cmd.Run()

	err = store.WithOpLock(func() error {
		os.Remove(pidFile)
		if len(livePids(home)) > 0 {
			return nil // another session of p goes on with the login
		}
		_, err := adoptSessionLogin(p, home)
		return err
	})
	restore()
	if err != nil {
		warn(i18n.T("warn.session_capture", err))
	}
	return passThrough(runErr)
}

// prepareSession links the shared setup, writes the session's global config
// and, unless another session of p is running (it holds the current token),
// gives the session the newest login: the profile's, or the one a session
// that ended without cswitch noticing (a closed terminal, a crash) left in
// the session directory.
func prepareSession(p *store.Profile, home claudecode.Home) error {
	if err := os.MkdirAll(pidDir(home), 0o700); err != nil {
		return err
	}
	shared := claudecode.Default.ConfigDir()
	for _, name := range sharedEntries {
		src, dst := filepath.Join(shared, name), filepath.Join(home.Dir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if _, err := os.Lstat(dst); err == nil {
			continue // already linked, or deliberately the session's own
		}
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}

	if err := writeSessionConfig(p, home); err != nil {
		return err
	}
	if len(livePids(home)) > 0 {
		return nil
	}
	adopted, err := adoptSessionLogin(p, home)
	if err != nil || adopted {
		return err // the session already holds the newest login
	}
	if p, err = store.Load(p.Name); err != nil {
		return err
	}
	if !p.HasCode {
		return i18n.E("err.no_secret_code", p.Name)
	}
	raw, err := store.Secret(p.Name, "code")
	if err != nil {
		return err
	}
	creds, err := claudecode.ParseCredentials(raw)
	if err != nil {
		return err
	}
	return home.WithCredentialsLock(func() error { return home.WriteCredentials(creds) })
}

// writeSessionConfig gives the session a global config: the default one's
// settings (so there is no onboarding) with the profile's account. MCP
// servers are refreshed from the default config on every launch.
func writeSessionConfig(p *store.Profile, home claudecode.Home) error {
	return home.WithConfigLock(func() error {
		def, err := fsutil.ReadJSONObject(claudecode.Default.ConfigPath())
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		cfg, err := fsutil.ReadJSONObject(home.ConfigPath())
		if os.IsNotExist(err) {
			cfg = maps.Clone(def)
			if cfg == nil {
				cfg = map[string]json.RawMessage{}
			}
		} else if err != nil {
			return err
		}
		if v, ok := def["mcpServers"]; ok {
			cfg["mcpServers"] = v
		}
		if len(p.OAuthAccount) > 0 {
			cfg["oauthAccount"] = p.OAuthAccount
		} else {
			delete(cfg, "oauthAccount")
		}
		return fsutil.WriteJSONObject(home.ConfigPath(), cfg)
	})
}

// adoptSessionLogin saves the login in p's session directory into the
// profile when it is newer than the saved one, and reports whether it did.
// A session that refreshed its token has made the profile's copy useless,
// so the session's must win; a login saved after the session ended (cswitch
// login, or a refresh by cswitch usage) is newer and stays. Each login or
// refresh moves the access token's expiry forward, so that decides.
func adoptSessionLogin(p *store.Profile, home claudecode.Home) (bool, error) {
	if _, err := os.Stat(home.Dir); err != nil {
		return false, nil // never ran
	}
	sess, err := home.ReadCredentials()
	if errors.Is(err, claudecode.ErrNotLoggedIn) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if raw, err := store.Secret(p.Name, "code"); err == nil {
		saved, perr := claudecode.ParseCredentials(raw)
		if perr == nil && saved.Raw == sess.Raw {
			return false, nil
		}
		sameLogin := perr == nil && saved.OAuth.RefreshToken == sess.OAuth.RefreshToken
		if perr == nil && !sameLogin && saved.OAuth.ExpiresAt >= sess.OAuth.ExpiresAt {
			return false, nil
		}
	}
	profileFixMu.Lock()
	defer profileFixMu.Unlock()
	if err := store.SetSecret(p.Name, "code", sess.Raw); err != nil {
		return false, err
	}
	fresh, err := store.Load(p.Name) // the cached profile is shared
	if err != nil {
		return false, err
	}
	if !fresh.HasCode {
		// An earlier version may have dropped the login as ended.
		fresh.HasCode = true
		if fresh.SubscriptionType == "" {
			fresh.SubscriptionType = sess.OAuth.SubscriptionType
		}
		if err := store.Save(fresh); err != nil {
			return false, err
		}
	}
	forgetUsage(p)
	return true, nil
}

// dropSession deletes p's session directory and its login (the keychain
// item is named after the directory's path, so it cannot be kept for a
// renamed profile). With keepLogin, a newer token in it is saved into the
// profile first. The caller holds the operation lock.
func dropSession(p *store.Profile, keepLogin bool) error {
	home := sessionHome(p)
	if _, err := os.Stat(home.Dir); err != nil {
		return nil
	}
	if sessionRunning(p) {
		return i18n.E("err.session_running", p.Name)
	}
	if keepLogin {
		if _, err := adoptSessionLogin(p, home); err != nil {
			return err
		}
	}
	if err := home.DeleteCredentials(); err != nil {
		return err
	}
	// RemoveAll removes the symlinks into ~/.claude, not what they point to.
	return os.RemoveAll(home.Dir)
}

// renameProfile renames a profile together with its session.
func renameProfile(oldName, newName string) error {
	p, err := store.Load(oldName)
	if err != nil {
		return err
	}
	if err := dropSession(p, true); err != nil {
		return err
	}
	return store.Rename(oldName, newName)
}

func mapCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "map [name] [dir]",
		Short:   i18n.T("cmd.map.short"),
		Example: "  cswitch map work ~/work\n  cswitch map work\n  cswitch map",
		Args:    cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := store.LoadMappings()
			if len(args) == 0 {
				if len(m) == 0 {
					info(i18n.T("info.no_mappings"))
					return nil
				}
				rows := [][]string{splitHeader(i18n.T("map.header"))}
				for _, d := range m.Dirs() {
					rows = append(rows, []string{d, m[d]})
				}
				printTable(os.Stdout, rows)
				return nil
			}
			p, err := store.Load(args[0])
			if err != nil {
				return err
			}
			dir := "."
			if len(args) == 2 {
				dir = args[1]
			}
			clean, err := store.CleanDir(dir)
			if err != nil {
				return err
			}
			if fi, err := os.Stat(clean); err != nil || !fi.IsDir() {
				return i18n.E("err.map_not_dir", dir)
			}
			m[clean] = p.Name
			if err := store.SaveMappings(m); err != nil {
				return err
			}
			info(i18n.T("info.mapped", clean, p.Name))
			return nil
		},
	}
}

func unmapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unmap [dir]",
		Short: i18n.T("cmd.unmap.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			clean, err := store.CleanDir(dir)
			if err != nil {
				return err
			}
			m := store.LoadMappings()
			if _, ok := m[clean]; !ok {
				return i18n.E("err.map_none", clean)
			}
			delete(m, clean)
			if err := store.SaveMappings(m); err != nil {
				return err
			}
			info(i18n.T("info.unmapped", clean))
			return nil
		},
	}
}
