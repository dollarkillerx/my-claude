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

`cswitch <name>` switches both apps; `--code-only` / `--desktop-only` switch one. The current logins are saved before every switch. Restart running `claude` sessions afterwards; until then they keep using the old account.

### Set the language

```bash
cswitch lang zh
```

The choice is remembered for every later run. `cswitch lang auto` follows the system again.

## Commands

| Command | Description |
| --- | --- |
| `cswitch` | List accounts with usage and pick one to switch to |
| `cswitch <name>` | Switch to an account (same as `cswitch use <name>`) |
| `cswitch login [name]` | Log in to another account without logging out. `--desktop`, `--desktop-only` |
| `cswitch usage [name...]` | Show plan usage. `--all` also shows internal entries |
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
