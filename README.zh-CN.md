# cswitch

[English](README.md) | **简体中文** | [日本語](README.ja.md)

用一条命令切换多个 Claude 账户：在 macOS 上 **Claude Code** 和 **Claude Desktop** 同时生效，在 Linux 上切换 Claude Code。

```console
$ cswitch list
使用中  名称      邮箱                  组织         套餐  已保存
CD      work      me@company.com        Company      max   code,desktop
-       personal  me@example.com        Personal     pro   code,desktop

$ cswitch use personal
正在退出 Claude Desktop…
已在 2 个账户目录之间同步 Desktop 会话：复制 3 个，移除 0 个
正在打开 Claude Desktop…
Claude Desktop → personal
Claude Code → personal（me@example.com）

$ cswitch usage
  personal  me@example.com  pro
    5h         [████░░░░░░░░░░░░░░░░]  21.0%  2 小时 14 分后重置（10-14 17:30）
    7d         [██████████░░░░░░░░░░]  48.0%  3 天 6 小时后重置（10-18 09:00）
* work      me@company.com  max
    5h         [█░░░░░░░░░░░░░░░░░░░]   4.0%  4 小时 40 分后重置（10-14 19:56）
    7d         [███░░░░░░░░░░░░░░░░░]  13.0%  5 天 1 小时后重置（10-20 04:00）
```

> **非官方工具。** cswitch 是社区项目，与 Anthropic 没有任何关联，也未经其认可。它读写的是应用的本地存储，存储格式可能随任何一次更新而改变。详见[局限](#局限)。

## 功能

- **多账户，一条命令切换。** 想保存多少个账户都可以，用 `cswitch use <名称>` 让两个应用同时切换。
- **Claude Code 和 Claude Desktop 同时生效。** 也可以用 `--code-only` / `--desktop-only` 只切换其中一个，两个应用不必登录同一个账户。
- **一眼看清额度。** `cswitch usage` 并发查询所有账户的 5 小时和每周额度，使用的是 Claude Code `/usage` 背后的同一个接口。
- **Desktop 历史共享。** 每次切换时，Claude Desktop 的本地会话（Code 标签页和 Cowork）会在所有账户之间合并，每个账户都能看到全部会话。
- **安全优先。**
  - 保存的登录状态存在 macOS 钥匙串里。
  - 修改任何文件前都会先备份；切换失败会自动回滚。
  - 当前登录如果还没有保存，cswitch 会拒绝覆盖它。
- **支持 English、简体中文、日本語。** 默认跟随系统语言，也可以用 `--lang` 指定。

## 支持的平台

| | Claude Code | Claude Desktop |
| --- | --- | --- |
| macOS（Apple Silicon 或 Intel） | ✅ | ✅ |
| Linux | ✅ | —（没有官方的 Linux 版 Claude Desktop） |

Claude Code 需要用 Claude 订阅账户登录（Pro / Max / Team / Enterprise），用 API key 登录的无法切换。

## 安装

从 [Releases 页面](https://github.com/dollarkillerx/my-claude/releases)下载二进制文件（macOS arm64、Linux x86-64）：

```bash
tar -xzf cswitch_*_darwin_arm64.tar.gz
```

```bash
sudo mv cswitch_*_darwin_arm64/cswitch /usr/local/bin/
```

在 macOS 上如果被 Gatekeeper 拦截，可以去掉隔离标记：

```bash
xattr -d com.apple.quarantine /usr/local/bin/cswitch
```

也可以用 Go 1.24 或更高版本从源码构建：

```bash
go install github.com/dollarkillerx/my-claude/cmd/cswitch@latest
```

## 快速开始

### 1. 保存当前登录的账户

```bash
cswitch add work
```

这会保存 Claude Code 当前登录的账户。如果 Claude Desktop 登录的是同一个账户，也会一并保存。为了完整地复制文件，Desktop 会短暂重启。

如果 Desktop 登录的是**另一个**账户，cswitch 会提示你。把它单独保存为另一个名字：

```bash
cswitch add personal --desktop-only
```

### 2. 添加更多账户

> ⚠️ **不要用应用自带的「登出」来添加账户。** 登出会在服务器端吊销 token，已保存的登录状态也会跟着失效。

`cswitch login` 可以在不登出的情况下登录另一个账户：

```bash
cswitch login personal
```

- **Claude Code**：先保存当前登录，再运行 `claude auth login` 登录新账户。
- **Claude Desktop**：退出 Desktop，只在本地清除登录状态（不会在服务器端登出），重新打开后等待你登录。

也可以用 `--code-only` 或 `--desktop-only` 只处理其中一边。如果某个 profile 只保存了一边，之后在对应应用里登录该账户，再运行一次 `cswitch add <名称>` 就能补齐。

### 3. 切换

```bash
cswitch use personal
```

切换后请重启正在运行的 `claude` 会话；在重启前它们仍然使用原来的账户。

## 命令一览

| 命令 | 说明 |
| --- | --- |
| `cswitch add <名称>` | 把当前登录保存为 profile，已存在则更新。`--code-only`、`--desktop-only` |
| `cswitch login <名称>` | 不登出当前账户，登录另一个账户并保存。`--code-only`、`--desktop-only`、`--force` |
| `cswitch use <名称>` | 同时切换两个应用（别名 `switch`）。`--code-only`、`--desktop-only`、`--no-launch`、`--force` |
| `cswitch list` | 列出 profile（别名 `ls`）。`C` = Claude Code 正在使用，`D` = Claude Desktop 正在使用 |
| `cswitch current` | 显示两个应用当前分别使用哪个 profile |
| `cswitch usage [名称...]` | 查看所有（或指定）profile 的额度 |
| `cswitch sync` | 立即在所有账户之间同步 Desktop 本地会话（每次切换时也会自动执行） |
| `cswitch remove <名称>` | 删除已保存的 profile（别名 `rm`），不会登出任何账户。`-y` |

全局选项：`--lang en|zh|ja`、`--version`。运行 `cswitch <命令> --help` 查看详细说明。

### `--force`

如果当前登录的账户还没有保存到任何 profile，切换走就会丢失它，所以 cswitch 默认会拒绝切换。加上 `--force` 则强制切换。

## 语言

按以下顺序决定显示语言：`--lang`、`CSWITCH_LANG`、`LC_ALL`、`LC_MESSAGES`、`LANG`、macOS 系统语言。

```bash
cswitch --lang ja list
```

```bash
export CSWITCH_LANG=zh
```

## 历史共享：哪些能共享，哪些不能

| | 是否共享 |
| --- | --- |
| Claude Desktop 本地会话：Code 标签页和 Cowork（`claude-code-sessions`、`local-agent-mode-sessions`） | ✅ 每次切换时合并 |
| Claude Code 对话记录（`~/.claude/projects`） | ✅ 本来就是共享的，不区分账户 |
| claude.ai 的聊天、Projects、Artifacts | ❌ 按账户保存在 Anthropic 的服务器上 |

同一个会话在多个账户中都存在时，以最新的那份为准。在某个账户里删除的会话会被移到 `~/.config/cswitch/backups/sessions-trash/`，不会再被复制回来。

## 工作原理

简要来说：

- **Claude Code**：钥匙串中的 `Claude Code-credentials`，以及 `~/.claude.json` 里的 `oauthAccount`。
- **Claude Desktop**：`config.json` 中加密的 `oauth:*` token、Cookie 数据库和 claude.ai 的网页存储。这些都按原始字节复制，从不解密。
- **已保存的 profile**：登录信息用 AES-256-GCM 加密后存成文件，密钥存在钥匙串里（service 为 `cswitch`）；元数据存在 `~/.config/cswitch`（权限 0700）。
- **每次切换前**：先把当前的登录状态存回对应的 profile，因为两个应用都会轮换 refresh token。

详见 [docs/how-it-works.md](docs/how-it-works.md)（英文）。

## 数据与备份

```
~/.config/cswitch/
├── profiles/<名称>/meta.json      # 账户信息（不含密钥）
├── profiles/<名称>/*.enc          # 加密保存的登录信息（密钥在钥匙串中）
├── profiles/<名称>/desktop/       # 该账户的 Desktop Cookie 和网页存储
├── backups/                       # cswitch 修改前的文件副本（每个文件保留最近 10 份）
│   └── sessions-trash/            # 在某个账户中删除的 Desktop 会话
└── sync-state.json                # 上次会话同步时的文件列表
```

需要手动恢复时，先退出两个应用，再从 `backups/` 把需要的文件复制回原处。

卸载时，对每个 profile 运行 `cswitch remove <名称>`，删除 `~/.config/cswitch`，再删除钥匙串中的 `cswitch` 条目（`security delete-generic-password -s cswitch -a master-key`）。

## 局限

- **Linux 上只支持 Claude Code。** Desktop 相关的命令和选项会提示 Desktop 不可用。Linux 没有钥匙串，所以保存的登录信息用 `~/.config/cswitch/master.key`（权限 0600）中的密钥加密，这和 Claude Code 在 Linux 上保护它自己的 `~/.claude/.credentials.json` 的方式相同。
- **不支持 `CLAUDE_CONFIG_DIR`。** cswitch 只认 Claude Code 的默认位置。
- **依赖没有公开文档的本地存储格式。** Claude Code 或 Claude Desktop 的更新可能导致 cswitch 失效。遇到无法识别的格式时，cswitch 会报错退出，不会写入文件。
- **正在运行的 `claude` 会话在重启前仍使用原来的账户。** 它们还可能把旧账户写回 `~/.claude.json`；cswitch 会根据钥匙串中的 token 识别出这种情况，并在下次切换时自动修正。
- **当前账户的额度：** 如果当前账户的 token 已过期，而 `claude` 正在运行，cswitch 不会刷新这个 token，否则会把正在运行的会话踢下线。请先用一下 `claude` 或退出它，再重试。

## 参与贡献

欢迎提交 bug 报告、翻译和 Pull Request，详见 [CONTRIBUTING.md](CONTRIBUTING.md)。安全问题请参阅 [SECURITY.md](SECURITY.md)。

## 许可证

[MIT](LICENSE)
