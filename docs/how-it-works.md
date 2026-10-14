# How cswitch works

This document describes where Claude Code and Claude Desktop keep their login state, and exactly what cswitch reads and writes. It is meant for contributors and for anyone who wants to check what the tool does before trusting it with their accounts.

All of this is based on observing the apps on macOS. None of it is a documented interface, so it can change with any app update. cswitch validates what it reads and stops with an error rather than guessing.

## Claude Code

| What | Where |
| --- | --- |
| OAuth credentials (macOS) | Keychain generic password, service `Claude Code-credentials`, account = your macOS user name |
| OAuth credentials (Linux) | `~/.claude/.credentials.json` (mode 0600), same JSON |
| Account info | `oauthAccount` in `~/.claude.json` |

The credentials item is a JSON document:

```json
{
  "claudeAiOauth": {
    "accessToken": "…",
    "refreshToken": "…",
    "expiresAt": 1760000000000,
    "scopes": ["…"],
    "subscriptionType": "max"
  }
}
```

`oauthAccount` holds the account and organization UUIDs, the email address, and similar fields.

**Switching Claude Code** replaces the keychain item and the `oauthAccount` key. The file is rewritten atomically, and every other key in `~/.claude.json` is left as it was. If `~/.claude.json` is a symlink (e.g. managed by a dotfiles repo), the link's target is written and the link is kept. If updating the file fails, the keychain is put back as it was, including deleting the item if Claude Code was logged out before. The whole credentials JSON is stored and restored as-is, so fields cswitch doesn't know about survive.

### Running sessions

A running `claude` process keeps its tokens and its copy of `~/.claude.json` in memory. After a switch it continues as the old account until restarted, and its next config save may write the old `oauthAccount` back. To handle that, cswitch identifies the live account by the keychain refresh token first:

1. If the token matches a saved profile, that profile is the live account. If `~/.claude.json` names someone else, it is repaired.
2. If no profile matches, because Claude Code has rotated the token since it was saved, cswitch falls back to `oauthAccount`.

## Claude Desktop

Desktop's data directory is `~/Library/Application Support/Claude`.

| What | Where |
| --- | --- |
| OAuth tokens | `config.json` keys starting with `oauth:` (e.g. `oauth:tokenCache`, `oauth:tokenCacheV2`) |
| Current account | `config.json` key `lastKnownAccountUuid` |
| Cookies | `Cookies`, `Cookies-journal` (SQLite) |
| Web storage of the claude.ai origin | `Local Storage/`, `Session Storage/`, `IndexedDB/`, `WebStorage/` |
| Local sessions | `claude-code-sessions/<accountUuid>/<orgUuid>/local_*.json`, `local-agent-mode-sessions/<accountUuid>/<orgUuid>/local_*` |

A token key whose value is empty (`null`, `""`, `{}`) doesn't count as a login. Desktop can blank its tokens on logout or expiry while keeping `lastKnownAccountUuid`, and saving that state would overwrite a good saved login.

The token values are encrypted with Electron's `safeStorage`, whose key is the `Claude Safe Storage` keychain item. That key is per machine, not per account, so cswitch saves and restores the encrypted strings byte for byte. It never reads `Claude Safe Storage` or decrypts anything.

**Switching Claude Desktop** works like this:

1. Quit Desktop (AppleScript `quit app "Claude"`, wait up to 20 s).
2. Take a checkpoint: a copy of `config.json` and the per-account files above.
3. Save the current login back into the profile that owns it. From the checkpoint until the end of this step list, Ctrl-C and termination signals are deferred, so an interrupt can't leave a half-switched Desktop. Checkpoints left behind by a crash are deleted on the next run.
4. Replace the `oauth:*` and `lastKnownAccountUuid` keys, and replace the cookie and web-storage files with the target profile's copies. The live ones are moved to `backups/` first, so a stale cookie journal is never replayed onto another account's database.
5. If any of this fails, restore the checkpoint.
6. Sync local sessions (below), then reopen Desktop.

When cswitch *saves* a Desktop login (`add`, `login`), it also quits Desktop first, so the SQLite and LevelDB files are copied while nothing is writing them.

## Profiles

```
keychain  service "cswitch", account "master-key"  → 32-byte AES key (hex); on Linux: ~/.config/cswitch/master.key
~/.config/cswitch/profiles/<name>/code.enc         → Claude Code credentials JSON, encrypted
~/.config/cswitch/profiles/<name>/desktop.enc      → {"accountUuid", "keys": {config.json auth keys}}, encrypted
~/.config/cswitch/profiles/<name>/meta.json        → account info, which halves are saved
~/.config/cswitch/profiles/<name>/desktop/         → cookies + web storage copies
```

Profile secrets can be several KB: a Desktop snapshot holds two encrypted token blobs. They are therefore encrypted with AES-256-GCM into `*.enc` files, and only the key lives in the keychain. The profile name and secret kind are bound in as associated data, so an encrypted file can't be swapped into another profile or slot, and tampering is detected. Profiles saved by early versions, which kept secrets in `<name>/code` / `<name>/desktop` keychain items, are still read and are moved into files the next time they are saved.

Keychain writes go through `/usr/bin/security`:

- **Small values** (anything whose command line fits in 4,000 bytes, such as the master key and usually Claude Code's credentials) are written by piping `add-generic-password -X <hex>` into `security -i`, so they never appear in the process list.
- **Larger values** have to go on the command line, because `security -i` silently truncates lines at about 4 KB. While `security` runs, other local users could see the value in the process list. Claude Code writes its own item the same way. Only Claude Code's credentials item can take this path; cswitch's own secrets are kept small (just the master key).
- **Every write is read back and compared**, and `security`'s output is never included in errors, because it can echo the secret.

Reads use `find-generic-password -g` rather than `-w`, because `-w` prints non-ASCII data as bare hex that can't be told apart from a secret that happens to look like hex.

### Token rotation

Both apps refresh their access tokens, and a refresh also replaces the refresh token. A profile saved a week ago may therefore hold a refresh token the server no longer accepts. That is why every `use` and `login` first saves the current logins back into their profiles.

Logging out in either app revokes the tokens server-side, so `cswitch login` never logs out. For Claude Code it runs `claude auth login`, which replaces the stored login. For Desktop it only removes the login locally.

## Usage

`cswitch usage` calls the endpoint Claude Code's `/usage` screen uses:

```
GET https://api.anthropic.com/api/oauth/usage
Authorization: Bearer <access token>
anthropic-beta: oauth-2025-04-20
```

Every top-level object in the response that has a `utilization` field is shown as a window. That includes `five_hour`, `seven_day` and `seven_day_opus`, and any window added later shows up without a code change.

Expired tokens are refreshed at `https://console.anthropic.com/v1/oauth/token` with Claude Code's public OAuth client ID, and the new tokens are saved:

- **Other profiles:** the token is refreshed and saved to the profile.
- **The active account:** it is identified by the keychain token, as above, not by `~/.claude.json` alone. Its token is read from Claude Code's own keychain item and refreshed only when no `claude` process is running, because a running process would still hold the old, now-revoked refresh token in memory.

"A running `claude`" means:

- a process whose executable is named `claude`, or
- `node` running `@anthropic-ai/claude-code/cli.js` (an npm install).

The `claude` that Claude Desktop bundles for its Code tab (`…/claude.app/Contents/MacOS/claude`) is ignored, because it gets its login from Desktop, not from Claude Code's keychain item.

## Session sync

Desktop scopes local sessions by `<accountUuid>/<orgUuid>`. The session files don't embed those IDs, and the transcripts they point to live in `~/.claude/projects`, which isn't per account. Copying the files between account folders therefore makes every account list every session.

On each sync, for `claude-code-sessions` and `local-agent-mode-sessions`:

1. List every `<accountUuid>/<orgUuid>` folder, ignoring non-UUID names like `skills-plugin`.
2. Collect every file under a `local_*` entry, keyed by its path relative to the folder.
3. Find deletions. `sync-state.json` records when the last sync ran and what each folder held after it. A file that was there and is now gone was deleted by the user, so it is moved to `backups/sessions-trash/<date>/` from every folder instead of being copied back. The exception is when some folder holds a copy modified after the last sync: the user kept working on it there, so the edit wins and the session is restored.
4. Copy the newest version of every remaining file to each folder that lacks it or holds an older one.

Only `local_*` entries are synced. Account-specific files such as `scheduled-tasks.json` or `cowork-clientdata-cache.json` are left alone.

## Backups

Before cswitch modifies `~/.claude.json`, `config.json`, or Desktop's cookie and storage files, it copies or moves the current version into `~/.config/cswitch/backups/` with a timestamp. It keeps the newest 10 of each.

## Environment variables

These are mostly for testing:

| Variable | Default |
| --- | --- |
| `CSWITCH_LANG` | system language |
| `CSWITCH_HOME` | `~/.config/cswitch` |
| `CSWITCH_CLAUDE_JSON` | `~/.claude.json` |
| `CSWITCH_DESKTOP_DIR` | `~/Library/Application Support/Claude` |
| `CSWITCH_DESKTOP_APP` | `Claude` |
| `CSWITCH_CLAUDE_PROC` | `claude` |
| `CSWITCH_CODE_KEYCHAIN_SERVICE` | `Claude Code-credentials` |
| `CSWITCH_KEYCHAIN_SERVICE` | `cswitch` |
| `CSWITCH_TOKEN_URL`, `CSWITCH_USAGE_URL` | the Anthropic endpoints above |
| `CSWITCH_CODE_CREDENTIALS_FILE` | keychain on macOS, `~/.claude/.credentials.json` on Linux |
| `CSWITCH_KEY_FILE` | keychain on macOS, `~/.config/cswitch/master.key` on Linux |
