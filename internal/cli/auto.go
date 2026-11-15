package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

// autoOptions configure the auto-switcher.
type autoOptions struct {
	Threshold       float64       // session line, percent
	WeeklyThreshold float64       // weekly / per-model line, percent
	Models          []string      // per-model weekly limits that also count
	Interval        time.Duration // between checks
	Cooldown        time.Duration // minimum time between switches
	DryRun          bool
}

// autoState persists what the switcher needs across runs and restarts.
type autoState struct {
	LastSwitch time.Time `json:"lastSwitch"`
}

func autoStatePath() string { return filepath.Join(store.Home(), "auto-state.json") }

func loadAutoState() autoState {
	var s autoState
	if data, err := os.ReadFile(autoStatePath()); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func saveAutoState(s autoState) {
	if data, err := json.Marshal(s); err == nil && os.MkdirAll(store.Home(), 0o700) == nil {
		_ = fsutil.AtomicWrite(autoStatePath(), data, 0o600)
	}
}

func autoCmd() *cobra.Command {
	var o autoOptions
	var once, install, uninstall, status bool
	var models string
	c := &cobra.Command{
		Use:   "auto",
		Short: i18n.T("cmd.auto.short"),
		Long:  i18n.T("cmd.auto.long"),
		Example: "  cswitch auto\n  cswitch auto --threshold 80 --weekly-threshold 97\n" +
			"  cswitch auto --install\n  cswitch auto --status\n  cswitch auto --uninstall",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.WeeklyThreshold == 0 {
				o.WeeklyThreshold = o.Threshold
			}
			if o.Threshold < 50 || o.Threshold > 100 || o.WeeklyThreshold < 50 || o.WeeklyThreshold > 100 {
				return i18n.E("err.auto_bad_threshold")
			}
			if o.Interval < 30*time.Second {
				o.Interval = 30 * time.Second // the usage endpoint is rate limited
			}
			for _, m := range strings.Split(models, ",") {
				if m = strings.TrimSpace(m); m != "" {
					o.Models = append(o.Models, m)
				}
			}
			switch {
			case install:
				return installAutoService(serviceArgs(cmd))
			case uninstall:
				return uninstallAutoService()
			case status:
				return autoServiceStatus()
			case once:
				return autoTick(cmd.Context(), o)
			}
			logf(i18n.T("auto.started", o.Threshold, o.WeeklyThreshold, o.Interval))
			for {
				rotateAutoLog()
				if err := autoTick(cmd.Context(), o); err != nil {
					logf(i18n.T("auto.tick_error", err))
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(o.Interval):
				}
			}
		},
	}
	c.Flags().Float64Var(&o.Threshold, "threshold", 90, i18n.T("flag.auto_threshold"))
	c.Flags().Float64Var(&o.WeeklyThreshold, "weekly-threshold", 0, i18n.T("flag.auto_weekly_threshold"))
	c.Flags().StringVar(&models, "model", "", i18n.T("flag.auto_model"))
	c.Flags().DurationVar(&o.Interval, "interval", time.Minute, i18n.T("flag.auto_interval"))
	c.Flags().DurationVar(&o.Cooldown, "cooldown", 5*time.Minute, i18n.T("flag.auto_cooldown"))
	c.Flags().BoolVar(&o.DryRun, "dry-run", false, i18n.T("flag.auto_dry_run"))
	c.Flags().BoolVar(&once, "once", false, i18n.T("flag.auto_once"))
	c.Flags().BoolVar(&install, "install", false, i18n.T("flag.auto_install"))
	c.Flags().BoolVar(&uninstall, "uninstall", false, i18n.T("flag.auto_uninstall"))
	c.Flags().BoolVar(&status, "status", false, i18n.T("flag.auto_status"))
	c.MarkFlagsMutuallyExclusive("install", "uninstall", "status", "once")
	return c
}

// serviceArgs are the flags the background service is started with: the
// ones given now, minus the service-management flags themselves.
func serviceArgs(cmd *cobra.Command) []string {
	args := []string{"auto"}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "install", "uninstall", "status", "once", "lang":
			return
		}
		args = append(args, "--"+f.Name+"="+f.Value.String())
	})
	return args
}

func logf(msg string) {
	fmt.Printf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

// pressure is how close a reading is to the lines that matter: the highest
// of session/threshold and weekly(+models)/weeklyThreshold, as a fraction
// (>= 1 means over a line). Unknown readings count as fully used.
func pressure(r *usage.Report, o autoOptions) float64 {
	if r == nil {
		return 2
	}
	p := 0.0
	if s := r.Session(); s != nil {
		p = max(p, s.Percent/o.Threshold)
	}
	if w := r.Weekly(); w != nil {
		p = max(p, w.Percent/o.WeeklyThreshold)
	}
	for _, l := range r.Limits {
		if l.Model == "" {
			continue
		}
		for _, m := range o.Models {
			if strings.EqualFold(m, l.Model) {
				p = max(p, l.Percent/o.WeeklyThreshold)
			}
		}
	}
	if len(r.Limits) == 0 { // older responses
		for _, w := range r.Windows {
			switch w.Name {
			case "five_hour":
				p = max(p, w.Utilization/o.Threshold)
			case "seven_day":
				p = max(p, w.Utilization/o.WeeklyThreshold)
			}
		}
	}
	return p
}

// autoMaxAge is the oldest reading a switch is decided on. Readings are
// normally at most a few minutes old; older ones are left over from a rate
// limit, and an account may have used its allowance up since.
const autoMaxAge = 15 * time.Minute

// autoTick checks once and switches Claude Code if needed. Usage is read
// without the operation lock, which only guards the decision and the switch:
// reading every account can take a while, and a switch typed meanwhile must
// not wait for it.
func autoTick(ctx context.Context, o autoOptions) error {
	var current *store.Profile
	if err := store.WithOpLock(func() (err error) { current, err = codeOwner(); return err }); err != nil {
		return err
	}
	if current == nil {
		logf(i18n.T("auto.not_logged_in"))
		return nil
	}
	ps, err := store.List()
	if err != nil {
		return err
	}
	readings := fetchUsage(ctx, ps, current.Name, 45*time.Second)

	cur := readings[current.Name]
	if cur.report == nil || cur.age > autoMaxAge {
		why := cur.err
		if why == nil {
			why = i18n.E("err.usage_stale", int(cur.age.Minutes()))
		}
		return i18n.E("err.auto_no_reading", current.Name, why)
	}
	if pressure(cur.report, o) < 1 {
		return nil // under the lines
	}

	type candidate struct {
		p        *store.Profile
		pressure float64
	}
	var cands []candidate
	for _, p := range ps {
		if p.Name == current.Name || !p.HasCode {
			continue
		}
		if r := readings[p.Name]; r.report != nil && r.age <= autoMaxAge {
			if pr := pressure(r.report, o); pr < 1 {
				cands = append(cands, candidate{p, pr})
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].pressure < cands[j].pressure })

	return store.WithOpLock(func() error {
		// The login may have changed while usage was read; then the next
		// check decides again with readings for the new one.
		now, err := codeOwner()
		if err != nil || now == nil || now.Name != current.Name {
			return err
		}
		var target *store.Profile
		for _, c := range cands {
			if !sessionRunning(c.p) { // a running session holds that login
				target = c.p
				break
			}
		}
		if target == nil {
			logf(i18n.T("auto.all_exhausted", current.Name))
			return nil
		}
		st := loadAutoState()
		if since := time.Since(st.LastSwitch); since < o.Cooldown {
			logf(i18n.T("auto.cooldown", current.Name, target.Name, int((o.Cooldown - since).Seconds())))
			return nil
		}
		if o.DryRun {
			logf(i18n.T("auto.would_switch", current.Name, target.Name))
			return nil
		}
		if err := switchTo(target, switchFlags{partFlags: partFlags{codeOnly: true}}); err != nil {
			return err
		}
		st.LastSwitch = time.Now()
		saveAutoState(st)
		logf(i18n.T("auto.switched", current.Name, target.Name))
		return nil
	})
}

// autoLogMax bounds auto.log; see rotateAutoLog.
var autoLogMax int64 = 2 << 20

// rotateAutoLog keeps auto.log small when it is this process's output, as
// it is for the service (launchd and systemd open it for appending, so it
// can be emptied in place): past autoLogMax, its content moves to auto.log.1
// and it starts over.
func rotateAutoLog() {
	path := autoLogPath()
	fi, err := os.Stat(path)
	if err != nil || fi.Size() < autoLogMax {
		return
	}
	out, err := os.Stdout.Stat()
	if err != nil || !os.SameFile(fi, out) {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || fsutil.AtomicWrite(path+".1", data, 0o600) != nil {
		return
	}
	_ = os.Truncate(path, 0)
}
