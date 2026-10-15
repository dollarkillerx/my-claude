package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

func usageCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:     "usage [name...]",
		Short:   i18n.T("cmd.usage.short"),
		Long:    i18n.T("cmd.usage.long"),
		Example: "  cswitch usage\n  cswitch usage work",
		RunE: func(cmd *cobra.Command, args []string) error {
			var ps []*store.Profile
			if len(args) == 0 {
				all, err := store.List()
				if err != nil {
					return err
				}
				ps = all
			} else {
				for _, n := range args {
					p, err := store.Load(n)
					if err != nil {
						return err
					}
					ps = append(ps, p)
				}
			}
			if len(ps) == 0 {
				info(i18n.T("info.no_profiles"))
				return nil
			}
			// The live profile is found by the keychain token, not ~/.claude.json
			// alone: a running claude session can write a stale account back,
			// and treating the real live account as an ordinary profile would
			// refresh its token under that session.
			liveName := ""
			if st, err := identifyCode(); err == nil && st.owner != nil {
				liveName = st.owner.Name
			}

			reports := make([]*usage.Report, len(ps))
			errs := make([]error, len(ps))
			var g errgroup.Group
			g.SetLimit(4)
			for i, p := range ps {
				g.Go(func() error {
					reports[i], errs[i] = profileUsage(cmd.Context(), p, p.Name == liveName)
					return nil
				})
			}
			g.Wait()

			for i, p := range ps {
				mark := " "
				if p.Name == liveName {
					mark = "*"
				}
				fmt.Printf("%s %s  %s  %s\n", mark, p.Name, orDash(p.Email), orDash(p.SubscriptionType))
				switch {
				case errs[i] != nil:
					fmt.Printf("    %v\n", errs[i])
				default:
					printReport(reports[i], all)
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, i18n.T("flag.usage_all"))
	return c
}

// printReport prints one account's limits the way the Claude apps show them:
// the current session, the week, and per-model weekly allowances. Responses
// without a "limits" list fall back to the older per-window fields. With all,
// every raw window (including internal codenamed ones) is listed too.
func printReport(r *usage.Report, all bool) {
	type row struct {
		label  string
		pct    float64
		resets time.Time
		active bool
	}
	var rows []row
	for _, l := range r.Limits {
		rows = append(rows, row{limitLabel(l), l.Percent, l.ResetsAt, l.Active || !l.ResetsAt.IsZero()})
	}
	if len(rows) == 0 || all {
		for _, w := range r.Windows {
			if len(r.Limits) == 0 && !all && !knownWindow(w.Name) {
				continue
			}
			rows = append(rows, row{windowLabel(w.Name), w.Utilization, w.ResetsAt, w.Utilization > 0 || !w.ResetsAt.IsZero()})
		}
	}
	if len(rows) == 0 {
		fmt.Println("    " + i18n.T("usage.no_limits"))
		return
	}
	width := 0
	for _, x := range rows {
		width = max(width, cellWidth.StringWidth(x.label))
	}
	for _, x := range rows {
		note := resetsIn(x.resets)
		if !x.active && x.resets.IsZero() {
			note = i18n.T("usage.not_started")
		}
		fmt.Printf("    %s  %s %5.1f%%  %s\n", cellWidth.FillRight(x.label, width), bar(x.pct), x.pct, note)
	}
}

// limitLabel names a limit like the apps do.
func limitLabel(l usage.Limit) string {
	switch {
	case l.Kind == "session" || l.Group == "session":
		return i18n.T("usage.label.session")
	case l.Model != "":
		return i18n.T("usage.label.weekly_model", l.Model)
	case l.Kind == "weekly_all" || l.Group == "weekly":
		return i18n.T("usage.label.weekly")
	}
	return l.Kind
}

// knownWindow reports whether a usage window is one of the documented limits.
// The endpoint also returns internal, codenamed entries (e.g. for experiments)
// that mean nothing to users; those are shown only with --all.
func knownWindow(name string) bool {
	switch name {
	case "five_hour", "seven_day", "seven_day_opus", "seven_day_sonnet", "seven_day_oauth_apps":
		return true
	}
	return false
}

// profileUsage fetches usage for p, refreshing its token when needed.
//
// The active account's token is read from Claude Code's own credentials
// (Claude Code may have rotated it since it was saved), and a refreshed token
// is written back there. That is exactly what one of several concurrent
// claude sessions does when it refreshes the shared login, and the other
// sessions pick the new token up from the keychain the same way.
func profileUsage(ctx context.Context, p *store.Profile, live bool) (*usage.Report, error) {
	if !p.HasCode {
		return nil, i18n.E("err.usage_needs_code")
	}
	var creds *claudecode.Credentials
	var err error
	if live {
		creds, err = claudecode.ReadCredentials()
	} else {
		var raw string
		if raw, err = store.Secret(p.Name, "code"); err == nil {
			creds, err = claudecode.ParseCredentials(raw)
		}
	}
	if err != nil {
		return nil, err
	}

	refresh := func() error {
		t, err := usage.Refresh(ctx, creds.OAuth.RefreshToken)
		if err != nil {
			return err
		}
		if creds, err = creds.WithTokens(t.AccessToken, t.RefreshToken, t.ExpiresAt); err != nil {
			return err
		}
		if err := store.SetSecret(p.Name, "code", creds.Raw); err != nil {
			return err
		}
		if live {
			return claudecode.WriteCredentials(creds)
		}
		return nil
	}

	refreshed := false
	if creds.Expired(2 * time.Minute) {
		if err := refresh(); err != nil {
			return nil, err
		}
		refreshed = true
	}
	r, err := usage.Fetch(ctx, creds.OAuth.AccessToken)
	if usage.IsUnauthorized(err) && !refreshed {
		if rerr := refresh(); rerr != nil {
			return nil, rerr
		}
		r, err = usage.Fetch(ctx, creds.OAuth.AccessToken)
	}
	if err != nil || live {
		return r, err // the live login's owner was verified by identifyCode
	}
	// Make sure the saved token is this profile's account: earlier versions
	// labeled tokens from ~/.claude.json, which can name the wrong account.
	owner, perr := usage.FetchProfile(ctx, creds.OAuth.AccessToken)
	if perr == nil && owner.AccountUUID != p.AccountUUID {
		profileFixMu.Lock()
		defer profileFixMu.Unlock()
		return nil, fixMislabeled(p, creds, owner)
	}
	return r, nil
}

// profileFixMu serializes profile repairs made from parallel usage queries.
var profileFixMu sync.Mutex

func windowLabel(name string) string {
	switch name {
	case "five_hour":
		return "5h"
	case "seven_day":
		return "7d"
	}
	if rest, ok := strings.CutPrefix(name, "seven_day_"); ok {
		return "7d " + rest
	}
	return name
}

func bar(pct float64) string {
	const width = 20
	n := int(pct/100*width + 0.5)
	n = max(0, min(width, n))
	return "[" + strings.Repeat("█", n) + strings.Repeat("░", width-n) + "]"
}

func resetsIn(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Until(t).Round(time.Minute)
	if d <= 0 {
		return i18n.T("usage.resets_now")
	}
	local := t.Local().Format("01-02 15:04")
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h >= 24 {
		return i18n.T("usage.resets_in_days", h/24, h%24, local)
	}
	return i18n.T("usage.resets_in", h, m, local)
}
