# Contributing to cswitch

Thanks for helping out! Bug reports, translations, docs fixes and code are all welcome.

## Reporting bugs

cswitch depends on undocumented local formats of Claude Code and Claude Desktop, so the most useful bug reports include:

- the output of `cswitch --version`, `claude --version`, and the Claude Desktop version (*Claude → About Claude*)
- your macOS version
- the command you ran and its full output

**Never paste tokens, cookies, or the contents of `~/.claude.json` or Desktop's `config.json` into an issue.** If you need to describe a file's structure, list its key names only.

## Development

You need Go 1.24+. macOS is needed for the Desktop and keychain code paths; everything else, including the Claude Code switch flow, also runs and is tested on Linux.

```bash
git clone https://github.com/dollarkillerx/my-claude.git
cd my-claude
make test
```

| Make target | What it does |
| --- | --- |
| `make build` | Build `bin/cswitch` |
| `make test` | `go vet` and unit tests |
| `make e2e` | End-to-end tests (see below) |
| `make install` | `go install` with the version stamped in |
| `make fmt` | `gofmt` all files |
| `make dist` | Release archives for macOS arm64 and Linux x86-64 in `dist/` |

### Project layout

```
cmd/cswitch/          entry point
internal/cli/         commands; ops.go holds the save/apply/rollback logic
internal/claudecode/  Claude Code keychain item and ~/.claude.json
internal/desktop/     Claude Desktop login state, process control, session sync
internal/store/       profiles (metadata on disk, encrypted secrets, key in the keychain)
internal/keychain/    wrapper around /usr/bin/security
internal/usage/       usage endpoint and token refresh
internal/i18n/        translations (en, zh, ja)
internal/fsutil/      atomic writes, copies, backups
docs/                 design notes
```

[docs/how-it-works.md](docs/how-it-works.md) explains what each part reads and writes.

### Tests

- **Unit tests** (`make test`) work in temporary directories. A few store and keychain tests create test-only keychain items with unique names and dummy values, and remove them afterwards. Your real Claude logins are never read or changed.
- **End-to-end tests** (`make e2e`, i.e. `go test -tags e2e ./internal/cli`) run the real commands against a sandbox: temporary config files, a fake Desktop folder, and test-only keychain items holding dummy values, removed afterwards. They don't read or change your real Claude logins, and they never quit the real Claude Desktop. They do need an unlocked login keychain, so they don't run in CI.

Please add or update tests with your change. For anything touching the switch flow, the e2e tests are the place.

### Manual testing

Every path cswitch touches can be redirected with environment variables, listed in [docs/how-it-works.md](docs/how-it-works.md#environment-variables). Use them to try a change without risking your real logins, for example:

```bash
export CSWITCH_HOME=/tmp/cs/home CSWITCH_CLAUDE_JSON=/tmp/cs/claude.json \
  CSWITCH_DESKTOP_DIR=/tmp/cs/desktop CSWITCH_DESKTOP_APP=nonexistent \
  CSWITCH_CODE_KEYCHAIN_SERVICE=cswitch-dev-code CSWITCH_KEYCHAIN_SERVICE=cswitch-dev
```

## Translations

All user-facing text lives in [`internal/i18n/messages.go`](internal/i18n/messages.go). Each key has an English, Chinese and Japanese entry.

- **Fixing a translation:** edit the string.
- **Adding a message:** add a key with all three languages, then use it with `i18n.T("key", args...)` or, for errors, `i18n.E("key", args...)`. `E` supports `%w`.
- **Keep format verbs consistent.** Every translation must use the same verbs (`%s`, `%d`, `%w`…) for the same arguments. If a language needs a different word order, use indexed verbs like `%[2]s`.
- **Adding a language:** add a `Lang` constant, append it to `Supported`, extend `Parse`, and fill in every key.
- **Update the docs too.** The three READMEs should say the same thing.

`go test ./internal/i18n` fails if a translation is missing, if the verbs don't match, or if the code uses a key that isn't in the catalog.

## Pull requests

- Keep changes focused; one topic per PR.
- Run `make fmt test` (and `make e2e` for switch-flow changes) before pushing.
- Never weaken the safety rules without discussing it in an issue first. These are:
  - back up before writing
  - refuse to overwrite an unsaved login without `--force`
  - roll back a failed Desktop switch
  - never log out
- Describe how you tested, including app versions if the change depends on their file formats.

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).
