# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

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

[0.1.0]: https://github.com/dollarkillerx/my-claude/releases/tag/v0.1.0
