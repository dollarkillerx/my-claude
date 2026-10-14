# Security

cswitch handles OAuth tokens and session cookies for your Claude accounts. This page describes how they are handled and how to report a problem.

## How secrets are handled

- **Saved logins are encrypted** with AES-256-GCM into `~/.config/cswitch/profiles/<name>/*.enc` (mode 0600). The key is kept in the macOS keychain (service `cswitch`, account `master-key`). On Linux, which has no keychain, it's kept in `~/.config/cswitch/master.key` (mode 0600). Each file is bound to its profile and slot, and tampering is detected.
- **Desktop cookies and web storage** are copied into `~/.config/cswitch/profiles/<name>/desktop/`. The directory has mode 0700 and the files 0600. The cookie values are encrypted by Claude Desktop with a per-machine key (`Claude Safe Storage`). cswitch never reads that key or decrypts anything, so the copies are useless on another machine.
- **Keychain writes:**
  - Small values (the master key, and usually Claude Code's credentials) are sent to `/usr/bin/security` over stdin, so they never appear in the process list.
  - Claude Code credentials longer than about 2 KB (for example with many MCP tokens) must go on `security`'s command line, because its stdin mode truncates long lines. While that `security` process runs, a fraction of a second, other local users could see the value in the process list. Claude Code writes the item the same way itself. cswitch's own secrets never take this path.
  - `security`'s output is never included in error messages, because it can echo the secret.
- **Interrupting is safe.** A Ctrl-C during a Desktop switch is deferred until the switch or its rollback has finished. Temporary copies left behind by a crash are deleted on the next run.
- **Network access** goes to Anthropic only, and only for `cswitch usage`:
  - `api.anthropic.com` to read usage
  - `console.anthropic.com` to refresh expired tokens
- **No telemetry.** cswitch sends nothing anywhere else.
- **Backups** in `~/.config/cswitch/backups/` can contain copies of `~/.claude.json`, Desktop's `config.json`, and its cookies, all with mode 0600. Delete that folder if you no longer need it.

Anyone who can run code as your macOS user can read all of this, just as they could read the apps' own storage. cswitch doesn't add a new trust boundary.

## Reporting a vulnerability

Please **do not open a public issue** for security problems. Use GitHub's private vulnerability reporting instead: *Security → Report a vulnerability* on the repository.

Include what you found, how to reproduce it, and its impact. You can expect an acknowledgement within a week.

Never include real tokens or cookies in a report. Redact them, or use the dummy values from the e2e tests.
