# cswitch

[English](README.md) | **简体中文** | [日本語](README.ja.md)

用一条命令切换多个 Claude 账户：在 macOS 上 **Claude Code** 和 **Claude Desktop** 同时生效，在 Linux 上切换 Claude Code。

```console
$ cswitch
#  名称      邮箱                  套餐  5小时  7天  使用中
1  personal  me@example.com        pro   21%    48%
2  work      me@company.com        max   4%     13%  * Code+Desktop
切换到（输入编号或名称，直接回车取消）：1
正在退出 Claude Desktop…
正在打开 Claude Desktop…
Claude Desktop → personal
Claude Code → personal（me@example.com）

$ cswitch work          # 也可以直接按名称切换
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
  - 切换前会自动保存当前的登录状态，不会丢失。
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

```bash
cswitch
```

就这一条：不带参数运行 `cswitch`，会列出所有账户和它们的额度，输入编号即可切换。当前登录的账户会自动保存，名称取自邮箱（`alex@example.com` 会保存为 `alex`）。只有 Desktop 登录的账户因为拿不到邮箱，会保存为 `desktop-<id>`，可以用 `cswitch rename desktop-1a2b3c4d work` 改名。

### 添加另一个账户

> ⚠️ **不要用应用自带的「登出」来添加账户。** 登出会在服务器端吊销 token，已保存的登录状态也会跟着失效。

```bash
cswitch login
```

它会先保存当前登录，再运行 `claude auth login` 让你登录另一个账户。完成后，两个账户都已保存。

- `cswitch login --desktop`：同时用新账户登录 Claude Desktop。cswitch 会退出 Desktop，只在本地清除登录状态（不会在服务器端登出），重新打开后等待你登录。
- `cswitch login --desktop-only`：只登录 Desktop。
- 想自己起名字，就加上名称，例如 `cswitch login work`。

### 切换

```bash
cswitch work
```

`cswitch <名称>` 会同时切换两个应用，`--code-only` / `--desktop-only` 只切换其中一个。每次切换前都会先保存当前的登录状态。切换后请重启正在运行的 `claude` 会话；在重启前它们仍使用原来的账户。

### 设置语言

```bash
cswitch lang zh
```

设置会被记住，之后每次运行都使用这个语言。`cswitch lang auto` 恢复为跟随系统。

## 命令一览

| 命令 | 说明 |
| --- | --- |
| `cswitch` | 列出所有账户及额度，选择一个切换 |
| `cswitch <名称>` | 切换到指定账户（等同于 `cswitch use <名称>`） |
| `cswitch login [名称]` | 不登出当前账户，登录另一个账户。`--desktop`、`--desktop-only` |
| `cswitch usage [名称...]` | 查看额度。`--all` 同时显示内部条目 |
| `cswitch list` | 列出已保存的账户（别名 `ls`） |
| `cswitch current` | 显示两个应用当前分别使用哪个账户 |
| `cswitch add [名称]` | 立即保存当前登录（一般用不到，切换时会自动保存） |
| `cswitch rename <旧名> <新名>` | 给账户改名（别名 `mv`） |
| `cswitch remove <名称>` | 删除已保存的账户（别名 `rm`），不会登出任何账户。`-y` |
| `cswitch lang [en\|zh\|ja\|auto]` | 查看或设置界面语言，设置后会一直使用 |
| `cswitch sync` | 立即在所有账户之间同步 Desktop 本地会话（每次切换时也会自动执行） |

切换相关的选项（用于 `cswitch <名称>` 和 `use`）：`--code-only`、`--desktop-only`、`--no-launch`、`--force`。`--force` 用于放弃一个无法识别账户的 Desktop 登录（默认会拒绝覆盖它）。全局选项：`--lang`、`--version`。

## 语言

按以下顺序决定显示语言：

1. `--lang`（只对当次运行有效）
2. `CSWITCH_LANG`
3. 用 `cswitch lang` 保存的语言
4. `LC_ALL`、`LC_MESSAGES`、`LANG`
5. macOS 系统语言

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
- **在 macOS 上通过 SSH 使用时，要先解锁钥匙串：** 运行 `security unlock-keychain ~/Library/Keychains/login.keychain-db`。登录钥匙串只有在屏幕前登录时才会自动解锁，`claude` 本身也有同样的限制。
- **不支持 `CLAUDE_CONFIG_DIR`。** cswitch 只认 Claude Code 的默认位置。
- **依赖没有公开文档的本地存储格式。** Claude Code 或 Claude Desktop 的更新可能导致 cswitch 失效。遇到无法识别的格式时，cswitch 会报错退出，不会写入文件。
- **正在运行的 `claude` 会话在重启前仍使用原来的账户。** 它们还可能把旧账户写回 `~/.claude.json`；cswitch 会根据钥匙串中的 token 识别出这种情况，并在下次切换时自动修正。
- **当前账户的额度：** 如果当前账户的 token 已过期，而 `claude` 正在运行，cswitch 不会刷新这个 token，否则会把正在运行的会话踢下线。请先用一下 `claude` 或退出它，再重试。

## 参与贡献

欢迎提交 bug 报告、翻译和 Pull Request，详见 [CONTRIBUTING.md](CONTRIBUTING.md)。安全问题请参阅 [SECURITY.md](SECURITY.md)。

## 许可证

[MIT](LICENSE)
