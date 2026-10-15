package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/dollarkillerx/my-claude/internal/desktop"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

// pickUsageTimeout bounds how long the picker waits for usage numbers; rows
// whose numbers have not arrived just show "-".
const pickUsageTimeout = 4 * time.Second

// pick shows every account with its usage and switches to the one chosen.
// Without a terminal it just prints the table.
func pick(ctx context.Context, sw switchFlags) error {
	// The current Claude Code login is saved first, so it always appears.
	live, err := codeOwner()
	if err != nil {
		warn(err.Error())
	}
	ps, err := store.List()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		info(i18n.T("info.no_profiles"))
		return nil
	}

	liveName := ""
	if live != nil {
		liveName = live.Name
	}
	reports := fetchUsage(ctx, ps, liveName, pickUsageTimeout)
	ps, _ = store.List() // usage checks may have repaired mislabeled profiles
	deskUUID := desktop.CurrentAccount()

	rows := [][]string{splitHeader(i18n.T("pick.header"))}
	for i, p := range ps {
		five, week := "-", "-"
		if r, ok := reports[p.Name]; ok && r.err == nil && r.report != nil {
			five, week = limitPct(r.report.Session()), limitPct(r.report.Weekly())
		}
		rows = append(rows, []string{strconv.Itoa(i + 1), p.Name, emailOrNote(p), orDash(p.SubscriptionType),
			five, week, activeLabel(p, liveName, deskUUID)})
	}
	printTable(os.Stdout, rows)

	if !isTerminal(os.Stdin) {
		return nil
	}
	fmt.Fprint(os.Stderr, "\n"+i18n.T("pick.prompt"))
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	choice := strings.TrimSpace(line)
	if choice == "" || choice == "q" {
		return nil
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(ps) {
			return i18n.E("err.bad_choice", choice)
		}
		return switchTo(ps[n-1], sw)
	}
	for _, p := range ps {
		if p.Name == choice {
			return switchTo(p, sw)
		}
	}
	return i18n.E("err.bad_choice", choice)
}

type usageResult struct {
	report *usage.Report
	err    error
}

// fetchUsage queries usage for every profile in parallel, giving up after
// timeout. It always waits for every query to finish before returning (an
// expired context aborts their requests quickly), so no refresh can write
// credentials after the caller has gone on to switch accounts.
func fetchUsage(ctx context.Context, ps []*store.Profile, liveName string, timeout time.Duration) map[string]usageResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out := make(map[string]usageResult, len(ps))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range ps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := profileUsage(ctx, p, p.Name == liveName)
			mu.Lock()
			out[p.Name] = usageResult{r, err}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func limitPct(l *usage.Limit) string {
	if l == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", l.Percent)
}

// activeLabel says which apps use p right now. The Claude Code side comes
// from the verified live login (liveName), not ~/.claude.json.
func activeLabel(p *store.Profile, liveName, deskUUID string) string {
	var on []string
	if liveName != "" && p.Name == liveName {
		on = append(on, "Code")
	}
	if desktop.Supported() && p.AccountUUID != "" && p.AccountUUID == deskUUID {
		on = append(on, "Desktop")
	}
	if len(on) == 0 {
		return ""
	}
	return "* " + strings.Join(on, "+")
}

// isTerminal reports whether f is an interactive terminal (not a pipe, a
// file, or /dev/null, which is also a character device).
func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }
