package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/dollarkillerx/my-claude/internal/claudecode"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

func usageCmd() *cobra.Command {
	return &cobra.Command{
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
				case len(reports[i].Windows) == 0:
					fmt.Println("    " + i18n.T("usage.no_limits"))
				default:
					for _, w := range reports[i].Windows {
						fmt.Printf("    %-10s %s %5.1f%%  %s\n", windowLabel(w.Name), bar(w.Utilization), w.Utilization, resetsIn(w.ResetsAt))
					}
				}
			}
			return nil
		},
	}
}

// profileUsage fetches usage for p, refreshing its token when needed.
//
// The active account is special: its token is read from Claude Code's own
// keychain item (Claude Code may have rotated it since it was saved), and it is
// only refreshed when no claude process is running. Refreshing rotates the
// refresh token, and a running claude process would still hold the old one in
// memory and get logged out on its next refresh.
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
		if live && claudecode.Running() {
			return i18n.E("err.usage_live_expired")
		}
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
	return r, err
}

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
