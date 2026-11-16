# cswitch

**English** | [简体中文](README.zh-CN.md) | [日本語](README.ja.md)

Switch between multiple Claude accounts with one command, in **Claude Code** and **Claude Desktop** at the same time (macOS), or in Claude Code (Linux).

```console
$ cswitch
#  NAME      EMAIL                 PLAN  5H   7D   IN USE
1  personal  me@example.com        pro   21%  48%
2  work      me@company.com        max   4%   13%  * Code+Desktop
Switch to (number or name, Enter to cancel): 1
Quitting Claude Desktop…
Opening Claude Desktop…
Claude Desktop → personal
Claude Code → personal (me@example.com)

$ cswitch work          # or switch directly by name
```

> **Unofficial.** cswitch is a community tool and is not affiliated with or endorsed by Anthropic. It works with the apps' local storage, whose format can change in any release. See [Limitations](#limitations).

## Features

- **Multiple accounts, one command.** Save as many accounts as you like and switch both apps with `cswitch use <name>`.
- **Claude Code and Claude Desktop together.** Each can also be switched on its own with `--code-only` / `--desktop-only`, and the two apps don't have to be on the same account.
- **Usage at a glance.** `cswitch usage` queries the 5-hour and weekly limits of every account in parallel, using the same endpoint as Claude Code's `/usage`.
- **Shared Desktop history.** Local Claude Desktop sessions (Code tab and Cowork) are merged across accounts on every switch, so each account sees all of them.
- **Safe by design.** Saved logins live in the macOS keychain. Every file is backed up before it is changed, a failed switch is rolled back, and the current logins are saved automatically before every switch, so none is ever lost.
- **English, 简体中文, 日本語.** Picked from your system language, or set with `--lang`.

## Platforms

| | Claude Code | Claude Desktop |
| --- | --- | --- |
| macOS (Apple Silicon or Intel) | ✅ | ✅ |
| Linux | ✅ | — (there is no official Claude Desktop for Linux) |

Claude Code must be logged in with a Claude subscription (Pro / Max / Team / Enterprise). API-key logins can't be switched.

## Install

Download a binary from the [releases page](https://github.com/dollarkillerx/my-claude/releases) (macOS arm64, Linux x86-64):

```bash
tar -xzf cswitch_*_darwin_arm64.tar.gz
```

```bash
sudo mv cswitch_*_darwin_arm64/cswitch /usr/local/bin/
```

On macOS, if Gatekeeper blocks the downloaded binary, clear the quarantine flag:

```bash
xattr -d com.apple.quarantine /usr/local/bin/cswitch
```

Or build from source with Go 1.24+:

```bash
go install github.com/dollarkillerx/my-claude/cmd/cswitch@latest
```

## Getting started

```bash
cswitch
```

That's it: running `cswitch` with no arguments lists your accounts with their usage and lets you pick one to switch to. The accounts you're logged in with are saved automatically, named after their email address (`alex@example.com` becomes `alex`). A Desktop-only account, whose email Desktop doesn't expose, is saved as `desktop-<id>`; give it a name with `cswitch rename desktop-1a2b3c4d work`.

### Add another account

> ⚠️ **Never use the apps' own "Log out" to add an account.** Logging out revokes the tokens on the server, and the saved login stops working.

```bash
cswitch login
```

This saves the current login, then runs `claude auth login` so you can sign in to another account. When you're done, both accounts are saved.

- `cswitch login --desktop` also signs in Claude Desktop with the new account. cswitch quits Desktop, clears its login locally (no server-side logout), reopens it, and waits for you to sign in.
- `cswitch login --desktop-only` does Desktop alone.
- Pass a name (`cswitch login work`) to choose the profile name yourself.

### Switch

```bash
cswitch work
```

`cswitch <name>` switches both apps; `--code-only` / `--desktop-only` switch one. The current logins are saved before every switch. Running `claude` sessions pick up the new account within about 30 seconds (Claude Code re-reads its login); restart them to switch right away.

### Set the language

```bash
cswitch lang zh
```

The choice is remembered for every later run. `cswitch lang auto` follows the system again.

### Limit resets

Some accounts get **limit resets**: free one-time refills of the weekly usage or of a model's allowance (the "Reset for free" buttons on the apps' usage page).

```bash
cswitch resets            # every account's resets: how many left, expiry, usable now?
cswitch reset work        # use one (the one Claude Code would pick); asks to confirm
cswitch reset work <id>   # use a specific one
```

A used reset can't be given back, so cswitch never uses one on its own. `cswitch usage` mentions when an account has resets available. When the server says Claude Code is too old for resets (`cli_version`), `resets` and `reset` run `claude update` and check again; `--no-update` turns that off.

### Automatic switching

```bash
cswitch auto                          # watch in this terminal
cswitch auto --threshold 80           # switch away at 80% (default 90%)
cswitch auto --weekly-threshold 95    # weekly: switch at 95% (default 99%)
cswitch auto --model Fable            # also count the Fable weekly limit
cswitch auto --install                # run in the background, started at login
cswitch auto --status                 # is it running? last switch?
cswitch auto --uninstall              # stop and remove it
```

When the active account's session or weekly usage reaches the threshold, cswitch switches Claude Code to the account with the most room left. It waits a cooldown (5 minutes) between switches and logs when no account has room. Only Claude Code is switched, because Claude Desktop would have to restart. The background service is a launchd LaunchAgent on macOS and a systemd user service on Linux; it logs to `~/.config/cswitch/auto.log`, which is kept under 2 MB (the previous part goes to `auto.log.1`). `--once` checks once, for cron, and `--dry-run` only logs what it would do.

### Several accounts at once, per directory

```bash
cswitch run work                 # Claude Code as "work" in this terminal only
cswitch run work --resume        # everything after the name goes to claude
cswitch map work ~/work          # bind a directory (and everything below it)
cd ~/work/project && cswitch run # → starts as "work"
cswitch map                      # list bindings; cswitch unmap ~/work removes one
```

`cswitch run` uses Claude Code's own `CLAUDE_CONFIG_DIR`: each account gets a session directory under `~/.config/cswitch/sessions/`, so other terminals and the global login are untouched and several accounts can work in parallel. Sessions share `~/.claude`'s settings, CLAUDE.md, skills, plugins and conversation history (so `--resume` works across accounts). The session's own token is saved back to the account when it exits, or, if the terminal was closed first, the next time cswitch uses the account. While a session runs, the global login won't be switched onto that account, since two copies of one login would invalidate each other. In an unbound directory, or for the account that is already the global login, `cswitch run` just starts `claude`.

### Usage readings and rate limits

The usage endpoint is itself rate limited, so readings are cached: accounts in use (Claude Code's, or one with a running `cswitch run` session) every minute; the others every 5 minutes. A 429 backs off for the server's `Retry-After` (or 1 → 30 minutes); meanwhile the last reading is shown with its age, and the rate limit is only mentioned once that reading is more than 15 minutes old. `cswitch usage --refresh` fetches now.

## Commands

| Command | Description |
| --- | --- |
| `cswitch` | List accounts with usage and pick one to switch to |
| `cswitch <name>` | Switch to an account (same as `cswitch use <name>`) |
| `cswitch login [name]` | Log in to another account without logging out. `--desktop`, `--desktop-only` |
| `cswitch usage [name...]` | Show plan usage. `--refresh` fetches now, `--all` adds internal entries |
| `cswitch resets [name...]` | Show limit resets |
| `cswitch reset <name> [id]` | Use one limit reset (asks to confirm; `-y` to skip) |
| `cswitch auto` | Switch automatically near a limit. `--install`, `--status`, `--uninstall`, `--once`, `--dry-run` |
| `cswitch run [name] [args]` | Start Claude Code as an account in this terminal only |
| `cswitch map [name] [dir]` / `unmap [dir]` | Bind a directory to an account for `run`, list or remove bindings |
| `cswitch list` | List saved accounts (alias `ls`) |
| `cswitch current` | Show which account each app is using |
| `cswitch add [name]` | Save the current logins now (rarely needed; switching saves them) |
| `cswitch rename <old> <new>` | Rename an account (alias `mv`) |
| `cswitch remove <name>` | Delete a saved account (alias `rm`). Nothing is logged out. `-y` |
| `cswitch lang [en\|zh\|ja\|auto]` | Show or set the language; the choice is remembered |
| `cswitch sync` | Share Desktop's local sessions across accounts now (also runs on every switch) |

Switch flags (for `cswitch <name>` and `use`): `--code-only`, `--desktop-only`, `--no-launch`, and `--force`. `--force` discards a Desktop login whose account can't be identified, which cswitch otherwise refuses to overwrite. Global flags: `--lang`, `--version`.

## Language

The language comes from, in order:

1. `--lang`, for one run
2. `CSWITCH_LANG`
3. the language saved with `cswitch lang`
4. `LC_ALL`, `LC_MESSAGES`, `LANG`
5. the macOS system language

## Shared history: what is and isn't shared

| | Shared? |
| --- | --- |
| Claude Desktop local sessions: Code tab and Cowork (`claude-code-sessions`, `local-agent-mode-sessions`) | ✅ Merged on every switch |
| Claude Code transcripts (`~/.claude/projects`) | ✅ Already shared; they aren't per account |
| claude.ai chats, Projects, artifacts | ❌ Stored on Anthropic's servers per account |

When the same session exists in several accounts, the newest copy wins. A session you delete in one account is moved to `~/.config/cswitch/backups/sessions-trash/` instead of being copied back.

## How it works

In short:

- **Claude Code:** the `Claude Code-credentials` keychain item, plus `oauthAccount` in `~/.claude.json`.
- **Claude Desktop:** the encrypted `oauth:*` tokens in `config.json`, plus the cookie database and the claude.ai web storage. These are copied byte for byte and never decrypted.
- **Saved profiles:** secrets are stored as AES-256-GCM encrypted files whose key is in the keychain (service `cswitch`). Metadata is in `~/.config/cswitch` (mode 0700).
- **Before every switch:** the current logins are saved back into their profiles, because both apps rotate their refresh tokens.

See [docs/how-it-works.md](docs/how-it-works.md) for the details.

## Data and backups

```
~/.config/cswitch/
├── profiles/<name>/meta.json      # account info (no secrets)
├── profiles/<name>/*.enc          # saved logins, encrypted (key in the keychain)
├── profiles/<name>/desktop/       # Desktop cookies + web storage for this account
├── backups/                       # copies of every file before cswitch changed it (newest 10 each)
│   └── sessions-trash/            # Desktop sessions deleted in one account
├── sessions/<name>/               # cswitch run: per-account CLAUDE_CONFIG_DIR
├── mappings.json                  # cswitch map: directory → account
├── usage-cache.json               # recent usage readings (rate control)
├── auto.log, auto-state.json      # cswitch auto: log (+ auto.log.1) and last switch
└── sync-state.json                # what the last session sync saw
```

To undo a change by hand, quit both apps and copy the file you need back from `backups/`.

To uninstall, run `cswitch remove <name>` for each profile, delete `~/.config/cswitch`, and remove the `cswitch` keychain item (`security delete-generic-password -s cswitch -a master-key`).

## Limitations

- **Linux: Claude Code only.** The Desktop commands and flags report that Desktop isn't available. Saved secrets are encrypted with a key in `~/.config/cswitch/master.key` (mode 0600), because there's no keychain. That's the same protection Claude Code gives its own `~/.claude/.credentials.json` there.
- **Over SSH on macOS, unlock the keychain first** with `security unlock-keychain ~/Library/Keychains/login.keychain-db`. The login keychain is only unlocked by a login at the screen; `claude` itself has the same limitation.
- **`CLAUDE_CONFIG_DIR` isn't supported.** cswitch expects Claude Code's default locations.
- **Depends on undocumented local formats.** A Claude Code or Claude Desktop update can break cswitch. If it finds something it doesn't recognize, it stops with an error instead of writing.
- **Running `claude` sessions keep the old account** until restarted. They may also write the old account back into `~/.claude.json`; cswitch detects this from the keychain token and repairs it on the next switch.
- **Usage for the active account:** if its token has expired while `claude` is running, cswitch won't refresh it, because that would log the running session out. Use `claude` for a moment, or quit it, then try again.

## Contributing

Bug reports, translations and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md). For security issues, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
