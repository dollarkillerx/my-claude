package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

// Limit resets ("上限のリセット"): grants that refill an account's usage
// once. Status and claims go through the same OAuth endpoints Claude Code
// uses; see usage/resets.go. Using one is never automatic.

func resetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "resets [name...]",
		Short:   i18n.T("cmd.resets.short"),
		Example: "  cswitch resets\n  cswitch reset alex",
		RunE: func(cmd *cobra.Command, args []string) error {
			ps, err := profilesFromArgs(args)
			if err != nil {
				return err
			}
			liveName := liveProfileName()
			readings := make([]reading, len(ps))
			var g errgroup.Group
			g.SetLimit(4)
			for i, p := range ps {
				g.Go(func() error {
					// Reset status changes only when one is granted or used, so a
					// fresh reading is not needed; the polling policy applies.
					readings[i] = readUsage(cmd.Context(), p, p.Name == liveName, false)
					return nil
				})
			}
			g.Wait()
			saveUsageCache()

			for i, p := range ps {
				fmt.Printf("%s  %s\n", p.Name, emailOrNote(p))
				rd := readings[i]
				switch {
				case rd.report == nil && rd.err != nil:
					fmt.Printf("    %v\n", rd.err)
				case rd.report == nil || rd.report.Resets == nil:
					fmt.Println("    " + i18n.T("resets.none_reported"))
				default:
					printResets(rd.report.Resets)
				}
			}
			return nil
		},
	}
}

// ineligibleText explains the server's reason code where it is known, and
// keeps the code itself either way.
func ineligibleText(reason string) string {
	key := "resets.why." + reason
	if reason == "" || i18n.T(key) == key {
		return orDash(reason)
	}
	return i18n.T(key) + " (" + reason + ")"
}

// printResets lists an account's grants.
func printResets(r *usage.Resets) {
	if !r.Eligible {
		fmt.Println("    " + i18n.T("resets.ineligible", ineligibleText(r.IneligibleReason)))
		return
	}
	if len(r.Grants) == 0 {
		fmt.Println("    " + i18n.T("resets.no_grants"))
		return
	}
	rows := [][]string{splitHeader(i18n.T("resets.header"))}
	for _, g := range r.Grants {
		state := i18n.T("resets.state.usable")
		switch {
		case g.Paused:
			state = i18n.T("resets.state.paused")
		case g.ResetsLeft == 0:
			state = i18n.T("resets.state.used_up")
		case !g.UsableNow && g.UseRequiresLimit:
			state = i18n.T("resets.state.needs_limit")
		case !g.UsableNow:
			state = i18n.T("resets.state.unavailable", orDash(g.UnavailableReason))
		}
		next := ""
		if g.ID == r.NextGrantID {
			next = " *"
		}
		rows = append(rows, []string{
			grantLabel(g) + next,
			fmt.Sprintf("%d/%d", g.ResetsLeft, g.ResetsTotal),
			formatExpiry(g.EndsAt),
			state,
			g.ID,
		})
	}
	var b strings.Builder
	printTable(&b, rows)
	for _, line := range strings.Split(strings.TrimRight(b.String(), "\n"), "\n") {
		fmt.Println("    " + line)
	}
	if !r.CooldownUntil.IsZero() && time.Now().Before(r.CooldownUntil) {
		fmt.Println("    " + i18n.T("resets.cooldown", r.CooldownUntil.Local().Format("01-02 15:04")))
	}
}

func grantLabel(g usage.Grant) string {
	if g.Label != "" {
		return g.Label
	}
	switch g.Type {
	case "weekly":
		return i18n.T("resets.type.weekly")
	case "session":
		return i18n.T("resets.type.session")
	}
	return g.ID
}

func formatExpiry(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func resetCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "reset <name> [grant-id]",
		Short:   i18n.T("cmd.reset.short"),
		Long:    i18n.T("cmd.reset.long"),
		Example: "  cswitch reset alex\n  cswitch reset alex example-grant",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(args[0])
			if err != nil {
				return err
			}
			live := liveProfileName() == p.Name
			// Use the current status, not a cached one: a claim is irreversible.
			rd := readUsage(cmd.Context(), p, live, true)
			saveUsageCache()
			if rd.report == nil {
				return rd.err
			}
			r := rd.report.Resets
			if r == nil || !r.Eligible || len(r.Grants) == 0 {
				return i18n.E("err.reset_none", p.Name)
			}
			id := r.NextGrantID
			if len(args) == 2 {
				id = args[1]
			}
			g := r.Grant(id)
			if g == nil {
				return i18n.E("err.reset_no_such_grant", orDash(id), p.Name)
			}
			if g.Paused || g.ResetsLeft == 0 {
				return i18n.E("err.reset_not_usable", grantLabel(*g))
			}
			if !r.CooldownUntil.IsZero() && time.Now().Before(r.CooldownUntil) {
				return i18n.E("err.reset_cooldown", r.CooldownUntil.Local().Format("01-02 15:04"))
			}
			if !g.UsableNow {
				if g.UseRequiresLimit {
					return i18n.E("err.reset_needs_limit", grantLabel(*g))
				}
				return i18n.E("err.reset_unavailable", grantLabel(*g), orDash(g.UnavailableReason))
			}

			fmt.Fprintln(os.Stderr, i18n.T("reset.confirm_details", p.Name, emailOrNote(p), grantLabel(*g), g.ResetsLeft, g.ResetsTotal, formatExpiry(g.EndsAt)))
			if !yes {
				fmt.Fprint(os.Stderr, i18n.T("prompt.reset"))
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
					return i18n.E("err.aborted")
				}
			}

			res, err := claimReset(cmd.Context(), p, live, g.ID)
			if err != nil {
				return err
			}
			// The next reading must show the refilled usage.
			forgetUsage(p)
			if res.OK() {
				left := strconv.Itoa(res.ResetsLeft)
				if res.ResetsLeft < 0 {
					left = "?"
				}
				info(i18n.T("info.reset_done", grantLabel(*g), p.Name, left))
				return nil
			}
			return i18n.E("err.reset_refused", res.Result, orDash(res.Reason))
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("flag.yes"))
	return c
}

// claimReset claims a grant with the token p's usage was just read with
// (see loginOf): the active account's as Claude Code holds it, a running
// session's, or the saved one.
func claimReset(ctx context.Context, p *store.Profile, live bool, grantID string) (*usage.ClaimResult, error) {
	creds, _, err := loginOf(p, live)
	if err != nil {
		return nil, err
	}
	if p.OrganizationUUID == "" {
		return nil, errors.New("no organization recorded for " + p.Name)
	}
	return usage.ClaimReset(ctx, creds.OAuth.AccessToken, p.OrganizationUUID, grantID)
}

// forgetUsage drops p's cached reading.
func forgetUsage(p *store.Profile) {
	cacheMu.Lock()
	dropCacheEntry(cacheKey(p))
	cacheMu.Unlock()
	saveUsageCache()
}

// liveProfileName is the profile holding Claude Code's login, or "".
func liveProfileName() string {
	if st, err := identifyCode(); err == nil && st.owner != nil {
		return st.owner.Name
	}
	return ""
}

// profilesFromArgs loads the named profiles, or all of them.
func profilesFromArgs(args []string) ([]*store.Profile, error) {
	if len(args) == 0 {
		ps, err := store.List()
		if err == nil && len(ps) == 0 {
			info(i18n.T("info.no_profiles"))
		}
		return ps, err
	}
	var ps []*store.Profile
	for _, n := range args {
		p, err := store.Load(n)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, nil
}
