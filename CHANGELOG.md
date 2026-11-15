# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Limit resets:** `cswitch resets` shows each account's resets: how many are left, when they expire, and whether they're usable now. `cswitch reset <name> [id]` uses one after a confirmation. Both use the same OAuth endpoints as Claude Code; cswitch never uses a reset on its own.
- **Auto-switch:** `cswitch auto` switches Claude Code to the account with the most room when the active one nears a limit. It has session, weekly and per-model thresholds, a cooldown, `--once` and `--dry-run`, and `--install` / `--status` / `--uninstall` for a launchd or systemd background service.
- **Session mode:** `cswitch run [name]` starts Claude Code as an account in one terminal through `CLAUDE_CONFIG_DIR`, sharing settings and history. `cswitch map` / `unmap` bind directories to accounts.
- **Usage rate control:** readings are cached (1 min for active or near-limit accounts, 5 min otherwise), 429s back off, and `usage --refresh` fetches now.

### Changed

- Credential swaps and `~/.claude.json` writes hold Claude Code's own lock files, so they never interleave with a running claude's token refresh. Running sessions now pick up a switch on their own within about 30 seconds.
- Tokens are refreshed at `platform.claude.com`, as Claude Code does.

### Fixed

- The Japanese README had lost its command list.
- Repairing a mislabeled login could race with parallel usage queries.

- **`login` said "not logged in with a Claude subscription" when Claude Code's login had ended.** Claude Code blanks its tokens when a login ends; that now counts as logged out, and `login` goes ahead even if the current login can't be saved.
- **The active account's token is no longer refreshed while `claude` runs.** Refreshing rotated the refresh token out from under running sessions, which then failed their own refresh and blanked Claude Code's login. (0.2.0 refreshed it regardless.)
- **A saved login the server no longer accepts** is removed with a message to log in again, instead of showing HTTP 400 on every run.
- OAuth error responses are shown readably.

## [0.2.0] - 2026-10-15

### Changed

- **`cswitch` alone opens an account picker** that lists every account with its usage; type a number or name to switch. `cswitch <name>` switches directly.
- **Current logins are saved automatically.** Switching and logging in no longer refuse to run because the current account isn't saved, and no separate `add` is needed. New accounts are named after their email; Desktop-only accounts are named `desktop-<id>`.
- **Names are optional** for `add` and `login`. Giving a name to an account saved under another name renames it.
- **`login` signs in Claude Code by default.** Use `--desktop` to also sign in Claude Desktop, or `--desktop-only`.
- Internal codenamed usage entries are hidden unless `--all` is given.
- **`list` shows Code and Desktop columns.** Tables align correctly with Chinese and Japanese text.

### Fixed

- **A login could be saved under the wrong account.** The account was read from `~/.claude.json`, which Claude Desktop's embedded Claude Code rewrites with Desktop's account. cswitch now asks the server who each token belongs to, and `usage` moves logins saved under the wrong name by 0.1 to the right account.
- A rename that failed halfway could leave a saved login undecryptable.
- The picker could let a background token refresh switch Claude Code back after you picked another account.
- `~/.claude.json` is created if missing instead of failing.
- A locked keychain (common over SSH) is explained, with the command to unlock it, instead of "exit status 36".

### Added

- `usage` and the picker show the limits the way the apps do, including per-model weekly limits (e.g. Fable), and mark windows that haven't started.
- Account names that match a command (`sync`, `ls`, …) are refused, and automatic names avoid them.
- `cswitch lang [en|zh|ja|auto]` saves the language for every later run.
- `cswitch rename <old> <new>`.
- An unknown name suggests the saved accounts and a likely mistyped command.

## [0.1.0] - 2026-10-14

### Added

- `add`, `login`, `use`, `list`, `current`, `usage`, `remove`, and `sync` commands.
- Switch Claude Code and Claude Desktop together, or one of them with `--code-only` / `--desktop-only`.
- Per-profile Claude Desktop cookies and web storage, so the UI never shows another account's cached data.
- Plan usage for every profile, fetched in parallel, with automatic token refresh.
- Claude Desktop local sessions shared across accounts, with deletions propagated to a trash folder.
- Safety features:
  - backups before every write
  - checkpoint and rollback for Desktop switches
  - refusal to overwrite unsaved logins without `--force`
  - repair of a stale `oauthAccount` written back by a running claude session
- English, Simplified Chinese, and Japanese, chosen from the system language or `--lang`.
- Linux support for Claude Code (credentials in `~/.claude/.credentials.json`); Desktop features are macOS-only.
- Profile secrets are encrypted with AES-256-GCM into files, with only the key in the keychain, so logins of any size can be saved.
- Ctrl-C during a Desktop switch is deferred until the switch or its rollback finishes, and leftover temporary copies are cleaned up.

[0.2.0]: https://github.com/dollarkillerx/my-claude/releases/tag/v0.2.0
[0.1.0]: https://github.com/dollarkillerx/my-claude/releases/tag/v0.1.0
