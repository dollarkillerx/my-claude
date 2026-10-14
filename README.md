# cswitch

**English** | [简体中文](README.zh-CN.md) | [日本語](README.ja.md)

Switch between multiple Claude accounts with one command, in **Claude Code** and **Claude Desktop** at the same time (macOS), or in Claude Code (Linux).

```console
$ cswitch list
ACTIVE  NAME      EMAIL                 ORG          PLAN  SAVED
CD      work      me@company.com        Company      max   code,desktop
-       personal  me@example.com        Personal     pro   code,desktop

$ cswitch use personal
Quitting Claude Desktop…
Synced Desktop sessions across 2 account folders: 3 copied, 0 removed
Opening Claude Desktop…
Claude Desktop → personal
Claude Code → personal (me@example.com)

$ cswitch usage
  personal  me@example.com  pro
    5h         [████░░░░░░░░░░░░░░░░]  21.0%  resets in 2h14m (10-14 17:30)
    7d         [██████████░░░░░░░░░░]  48.0%  resets in 3d6h (10-18 09:00)
* work      me@company.com  max
    5h         [█░░░░░░░░░░░░░░░░░░░]   4.0%  resets in 4h40m (10-14 19:56)
    7d         [███░░░░░░░░░░░░░░░░░]  13.0%  resets in 5d1h (10-20 04:00)
```

> **Unofficial.** cswitch is a community tool and is not affiliated with or endorsed by Anthropic. It works with the apps' local storage, whose format can change in any release. See [Limitations](#limitations).

## Features

- **Multiple accounts, one command.** Save as many accounts as you like and switch both apps with `cswitch use <name>`.
- **Claude Code and Claude Desktop together.** Each can also be switched on its own with `--code-only` / `--desktop-only`, and the two apps don't have to be on the same account.
- **Usage at a glance.** `cswitch usage` queries the 5-hour and weekly limits of every account in parallel, using the same endpoint as Claude Code's `/usage`.
- **Shared Desktop history.** Local Claude Desktop sessions (Code tab and Cowork) are merged across accounts on every switch, so each account sees all of them.
- **Safe by design.** Saved logins live in the macOS keychain. Every file is backed up before it is changed, a failed switch is rolled back, and cswitch refuses to overwrite a login you haven't saved.
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

### 1. Save the accounts you're logged in with now

```bash
cswitch add work
```

This saves the account Claude Code is logged in as. If Claude Desktop is logged in as the same account, its login is saved too. Desktop restarts briefly so its files can be copied consistently.

If Desktop is logged in as a **different** account, cswitch tells you. Save it under its own name:

```bash
cswitch add personal --desktop-only
```

### 2. Add more accounts

> ⚠️ **Never use the apps' own "Log out" to add an account.** Logging out revokes the tokens on the server, and the login you saved stops working.

`cswitch login` logs in to another account without logging out:

```bash
cswitch login personal
```

- **Claude Code:** saves the current login, then runs `claude auth login` for the new account.
- **Claude Desktop:** quits Desktop, clears its login locally (no server-side logout), reopens it, and waits for you to sign in.

You can also do one side at a time with `--code-only` or `--desktop-only`. To fill in the missing half of a profile later, log in to that account in the app and run `cswitch add <name>` again.

### 3. Switch

```bash
cswitch use personal
```

Restart any running `claude` sessions afterwards; until then they keep using the old account.

## Commands

| Command | Description |
| --- | --- |
| `cswitch add <name>` | Save the current login as a profile, or update it. `--code-only`, `--desktop-only` |
| `cswitch login <name>` | Log in to another account without logging out, and save it. `--code-only`, `--desktop-only`, `--force` |
| `cswitch use <name>` | Switch both apps (alias `switch`). `--code-only`, `--desktop-only`, `--no-launch`, `--force` |
| `cswitch list` | List profiles (alias `ls`). `C` = active in Claude Code, `D` = active in Claude Desktop |
| `cswitch current` | Show which profile each app is using |
| `cswitch usage [name...]` | Show plan usage of all or the given profiles |
| `cswitch sync` | Share Desktop's local sessions across accounts now (also runs on every switch) |
| `cswitch remove <name>` | Delete a saved profile (alias `rm`). Nothing is logged out. `-y` |

Global flags: `--lang en|zh|ja`, `--version`. Run `cswitch <command> --help` for details.

### `--force`

cswitch refuses to switch away from a login that isn't saved in any profile, because that login would be lost. `--force` switches anyway.

## Language

The language comes from, in order: `--lang`, `CSWITCH_LANG`, `LC_ALL`, `LC_MESSAGES`, `LANG`, and the macOS system language.

```bash
cswitch --lang ja list
```

```bash
export CSWITCH_LANG=zh
```

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
- **`CLAUDE_CONFIG_DIR` isn't supported.** cswitch expects Claude Code's default locations.
- **Depends on undocumented local formats.** A Claude Code or Claude Desktop update can break cswitch. If it finds something it doesn't recognize, it stops with an error instead of writing.
- **Running `claude` sessions keep the old account** until restarted. They may also write the old account back into `~/.claude.json`; cswitch detects this from the keychain token and repairs it on the next switch.
- **Usage for the active account:** if its token has expired while `claude` is running, cswitch won't refresh it, because that would log the running session out. Use `claude` for a moment, or quit it, then try again.

## Contributing

Bug reports, translations and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md). For security issues, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
