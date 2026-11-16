package cli

import (
	"context"
	"os"
	"os/exec"

	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

// The server turns down limit resets with "cli_version" when Claude Code is
// too old for them. Then `resets` and `reset` run `claude update` once
// (unless --no-update) and read the status again.

const reasonCLIVersion = "cli_version"

func needsClaudeUpdate(r *usage.Report) bool {
	return r != nil && r.Resets != nil && !r.Resets.Eligible && r.Resets.IneligibleReason == reasonCLIVersion
}

// updateClaudeCode runs `claude update`, with its output on stderr so that
// stdout stays cswitch's own.
func updateClaudeCode(ctx context.Context) error {
	info(i18n.T("info.claude_updating"))
	cmd := exec.CommandContext(ctx, "claude", "update")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return i18n.E("err.claude_update", err)
	}
	info(i18n.T("info.claude_updated"))
	return nil
}
