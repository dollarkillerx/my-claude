package i18n

// messages maps a key to its translations. Keep the format verbs (%s, %d,
// %v, %w, ...) identical and in the same order across languages.
var messages = map[string]map[Lang]string{
	// ---- prefixes ----
	"prefix.error": {
		EN: "error: ",
		ZH: "错误：",
		JA: "エラー: ",
	},
	"prefix.warning": {
		EN: "warning: ",
		ZH: "警告：",
		JA: "警告: ",
	},

	// ---- cobra help ----
	"help.usage":               {EN: "Usage:", ZH: "用法：", JA: "使い方:"},
	"help.aliases":             {EN: "Aliases:", ZH: "别名：", JA: "エイリアス:"},
	"help.examples":            {EN: "Examples:", ZH: "示例：", JA: "例:"},
	"help.commands":            {EN: "Available Commands:", ZH: "可用命令：", JA: "利用可能なコマンド:"},
	"help.additional_commands": {EN: "Additional Commands:", ZH: "其他命令：", JA: "その他のコマンド:"},
	"help.global_flags":        {EN: "Global Flags:", ZH: "全局选项：", JA: "グローバルオプション:"},
	"help.flags":               {EN: "Flags:", ZH: "选项：", JA: "オプション:"},
	"help.topics":              {EN: "Additional help topics:", ZH: "其他帮助主题：", JA: "その他のヘルプトピック:"},
	"help.more": {
		EN: `Use "{{.CommandPath}} [command] --help" for more information about a command.`,
		ZH: `运行 "{{.CommandPath}} [command] --help" 查看命令的详细说明。`,
		JA: `コマンドの詳細は "{{.CommandPath}} [command] --help" を実行してください。`,
	},
	"cmd.help.short": {
		EN: "Help about any command",
		ZH: "查看任意命令的帮助",
		JA: "コマンドのヘルプを表示",
	},

	// ---- flags ----
	"flag.help":         {EN: "help for %s", ZH: "%s 的帮助", JA: "%s のヘルプ"},
	"flag.version":      {EN: "print the version", ZH: "显示版本号", JA: "バージョンを表示"},
	"flag.lang":         {EN: "language: en, zh or ja (default: from the system locale)", ZH: "界面语言：en、zh 或 ja（默认跟随系统语言）", JA: "表示言語: en、zh、ja（既定はシステムの言語）"},
	"flag.code_only":    {EN: "only Claude Code", ZH: "只处理 Claude Code", JA: "Claude Code のみ"},
	"flag.desktop_only": {EN: "only Claude Desktop", ZH: "只处理 Claude Desktop", JA: "Claude Desktop のみ"},
	"flag.no_launch": {
		EN: "do not open Claude Desktop afterwards if it was not running",
		ZH: "如果 Claude Desktop 原本没有运行，切换后不要打开它",
		JA: "Claude Desktop が起動していなかった場合、切り替え後に起動しない",
	},
	"flag.yes": {EN: "do not ask for confirmation", ZH: "不再确认，直接执行", JA: "確認せずに実行する"},

	// ---- command help ----
	"cmd.root.short": {
		EN: "Manage and switch between multiple Claude accounts (Claude Code + Claude Desktop)",
		ZH: "管理并切换多个 Claude 账户（Claude Code + Claude Desktop）",
		JA: "複数の Claude アカウントを管理・切り替え（Claude Code + Claude Desktop）",
	},
	"cmd.current.short": {
		EN: "Show which profile Claude Code and Claude Desktop are using",
		ZH: "显示 Claude Code 和 Claude Desktop 当前使用的 profile",
		JA: "Claude Code と Claude Desktop が使用中のプロファイルを表示",
	},
	"cmd.use.short": {
		EN: "Switch Claude Code and Claude Desktop to a profile",
		ZH: "把 Claude Code 和 Claude Desktop 切换到指定 profile",
		JA: "Claude Code と Claude Desktop をプロファイルに切り替え",
	},
	"cmd.usage.short": {
		EN: "Show plan usage for all (or the given) profiles",
		ZH: "查看所有（或指定）profile 的额度使用情况",
		JA: "全（または指定した）プロファイルの使用量を表示",
	},
	"cmd.remove.short": {
		EN: "Delete a saved profile (does not log anything out)",
		ZH: "删除已保存的 profile（不会登出任何账户）",
		JA: "保存済みプロファイルを削除（ログアウトはしません）",
	},
	"cmd.sync.short": {
		EN: "Share Claude Desktop's local sessions across all accounts now",
		ZH: "立即在所有账户之间同步 Claude Desktop 的本地会话",
		JA: "Claude Desktop のローカルセッションを今すぐ全アカウントで共有",
	},
	"cmd.sync.long": {
		EN: `Merge Claude Desktop's local sessions (Code tab and Cowork) across the folders of
all accounts, keeping the newest copy of each. A session deleted in one account
is moved to the backup trash instead of being copied back. This also runs on
every switch. Claude Desktop is restarted if it is running.

Conversations on claude.ai are stored on Anthropic's servers per account and
cannot be shared.`,
		ZH: `把 Claude Desktop 的本地会话（Code 标签页和 Cowork）在所有账户的目录之间合并，
每个会话保留最新的一份。在某个账户中删除的会话会移到备份回收目录，不会被重新
复制回来。每次切换账户时也会自动执行。如果 Claude Desktop 正在运行，会重启它。

claude.ai 上的对话按账户保存在 Anthropic 的服务器上，无法共享。`,
		JA: `Claude Desktop のローカルセッション（Code タブと Cowork）を全アカウントのフォルダ間で
統合し、それぞれ最新のコピーを残します。あるアカウントで削除したセッションは
コピーし直されず、バックアップのゴミ箱へ移動されます。切り替えのたびにも
自動で実行されます。Claude Desktop が起動中なら再起動します。

claude.ai の会話はアカウントごとに Anthropic のサーバーに保存されているため、
共有できません。`,
	},

	// ---- output ----
	"current.not_logged_in": {EN: "not logged in", ZH: "未登录", JA: "未ログイン"},
	"current.not_saved":     {EN: "%s (not saved)", ZH: "%s（未保存）", JA: "%s（未保存）"},
	"usage.no_limits":       {EN: "no usage limits reported", ZH: "没有返回额度信息", JA: "使用量の上限は報告されていません"},
	"usage.resets_now":      {EN: "resets now", ZH: "即将重置", JA: "まもなくリセット"},
	"usage.resets_in": {
		EN: "resets in %dh%02dm (%s)",
		ZH: "%d 小时 %02d 分后重置（%s）",
		JA: "%d 時間 %02d 分後にリセット（%s）",
	},
	"usage.resets_in_days": {
		EN: "resets in %dd%dh (%s)",
		ZH: "%d 天 %d 小时后重置（%s）",
		JA: "%d 日 %d 時間後にリセット（%s）",
	},

	// ---- info ----
	"info.no_profiles": {
		EN: "No profiles yet. Run `cswitch add <name>` to save the current account.",
		ZH: "还没有 profile。运行 `cswitch add <名称>` 保存当前账户。",
		JA: "プロファイルはまだありません。`cswitch add <名前>` で現在のアカウントを保存してください。",
	},
	"info.saved_code": {
		EN: "Saved Claude Code login for %s as %q",
		ZH: "已将 %s 的 Claude Code 登录保存为 %q",
		JA: "%s の Claude Code ログインを %q として保存しました",
	},
	"info.saved_desktop": {
		EN: "Saved Claude Desktop login as %q",
		ZH: "已将 Claude Desktop 登录保存为 %q",
		JA: "Claude Desktop のログインを %q として保存しました",
	},
	"info.desktop_restart_to_save": {
		EN: "Claude Desktop will restart briefly so its login can be copied consistently.",
		ZH: "Claude Desktop 会短暂重启，以便完整地复制登录状态。",
		JA: "ログイン状態を一貫してコピーするため、Claude Desktop を一時的に再起動します。",
	},
	"info.desktop_quitting":    {EN: "Quitting Claude Desktop…", ZH: "正在退出 Claude Desktop…", JA: "Claude Desktop を終了しています…"},
	"info.desktop_opening":     {EN: "Opening Claude Desktop…", ZH: "正在打开 Claude Desktop…", JA: "Claude Desktop を起動しています…"},
	"info.desktop_rolled_back": {EN: "Claude Desktop was restored to its previous state.", ZH: "已将 Claude Desktop 恢复到切换前的状态。", JA: "Claude Desktop を元の状態に戻しました。"},
	"info.synced": {
		EN: "Synced Desktop sessions across %d account folders: %d copied, %d removed",
		ZH: "已在 %d 个账户目录之间同步 Desktop 会话：复制 %d 个，移除 %d 个",
		JA: "%d 個のアカウントフォルダ間で Desktop セッションを同期しました: コピー %d 件、削除 %d 件",
	},
	"info.interrupted": {
		EN: "Interrupted; Claude Desktop was left in a consistent state.",
		ZH: "已中断；Claude Desktop 已处于一致的状态。",
		JA: "中断しました。Claude Desktop は一貫した状態に保たれています。",
	},
	"info.in_sync": {EN: "Sessions are in sync", ZH: "会话已同步", JA: "セッションは同期済みです"},
	"info.skip_code": {
		EN: "Profile %s has no Claude Code login; leaving Claude Code unchanged",
		ZH: "profile %s 没有保存 Claude Code 登录，Claude Code 保持不变",
		JA: "プロファイル %s には Claude Code のログインがないため、Claude Code は変更しません",
	},
	"info.skip_desktop": {
		EN: "Profile %s has no Claude Desktop login; leaving Claude Desktop unchanged",
		ZH: "profile %s 没有保存 Claude Desktop 登录，Claude Desktop 保持不变",
		JA: "プロファイル %s には Claude Desktop のログインがないため、Claude Desktop は変更しません",
	},
	"info.already_code":     {EN: "Claude Code is already using %s", ZH: "Claude Code 已经在使用 %s", JA: "Claude Code はすでに %s を使用しています"},
	"info.already_desktop":  {EN: "Claude Desktop is already using %s", ZH: "Claude Desktop 已经在使用 %s", JA: "Claude Desktop はすでに %s を使用しています"},
	"info.switched_code":    {EN: "Claude Code → %s (%s)", ZH: "Claude Code → %s（%s）", JA: "Claude Code → %s（%s）"},
	"info.switched_desktop": {EN: "Claude Desktop → %s", ZH: "Claude Desktop → %s", JA: "Claude Desktop → %s"},
	"info.removed":          {EN: "Removed profile %s", ZH: "已删除 profile %s", JA: "プロファイル %s を削除しました"},

	// ---- prompts ----
	"prompt.desktop_login": {
		EN: "Sign in to Claude Desktop, then press Enter here… ",
		ZH: "请在 Claude Desktop 中登录，完成后在这里按回车… ",
		JA: "Claude Desktop でサインインしてから、ここで Enter を押してください… ",
	},
	"prompt.desktop_login_as": {
		EN: "Sign in to Claude Desktop as %s, then press Enter here… ",
		ZH: "请在 Claude Desktop 中用 %s 登录，完成后在这里按回车… ",
		JA: "Claude Desktop に %s でサインインしてから、ここで Enter を押してください… ",
	},
	"prompt.remove": {
		EN: "Delete profile %s (%s)? [y/N] ",
		ZH: "删除 profile %s（%s）？[y/N] ",
		JA: "プロファイル %s（%s）を削除しますか？ [y/N] ",
	},

	// ---- warnings ----
	"warn.claude_running": {
		EN: "claude is running. Restart those sessions: until then they keep the old account and may write it back to ~/.claude.json (cswitch repairs that on the next switch).",
		ZH: "有 claude 进程正在运行。请重启这些会话：在重启前它们仍使用原来的账户，并可能把旧账户写回 ~/.claude.json（下次切换时 cswitch 会自动修正）。",
		JA: "claude が実行中です。それらのセッションを再起動してください。再起動までは元のアカウントのままで、~/.claude.json に書き戻すことがあります（次の切り替え時に cswitch が修正します）。",
	},
	"warn.sync_failed": {EN: "session sync failed: %v", ZH: "会话同步失败：%v", JA: "セッションの同期に失敗しました: %v"},

	// ---- errors ----
	"err.bad_lang": {
		EN: "unsupported language %q (use en, zh or ja)",
		ZH: "不支持的语言 %q（可用 en、zh 或 ja）",
		JA: "サポートされていない言語 %q（en、zh、ja のいずれか）",
	},
	"err.aborted": {EN: "aborted", ZH: "已取消", JA: "中止しました"},
	"err.invalid_name": {
		EN: "invalid profile name %q (use letters, digits, '.', '_' or '-')",
		ZH: "无效的 profile 名称 %q（只能使用字母、数字、'.'、'_' 或 '-'）",
		JA: "無効なプロファイル名 %q（英数字、'.'、'_'、'-' のみ使用できます）",
	},
	"err.no_profile": {
		EN: "profile %s does not exist (%w)",
		ZH: "profile %s 不存在（%w）",
		JA: "プロファイル %s は存在しません（%w）",
	},
	"err.profile_corrupt": {
		EN: "profile %s is unreadable: %w",
		ZH: "profile %s 无法读取：%w",
		JA: "プロファイル %s を読み取れません: %w",
	},
	"err.no_secret_code": {
		EN: "profile %s has no saved Claude Code login",
		ZH: "profile %s 没有保存 Claude Code 登录",
		JA: "プロファイル %s には Claude Code のログインが保存されていません",
	},
	"err.no_secret_desktop": {
		EN: "profile %s has no saved Claude Desktop login",
		ZH: "profile %s 没有保存 Claude Desktop 登录",
		JA: "プロファイル %s には Claude Desktop のログインが保存されていません",
	},
	"err.no_oauth_account": {
		EN: "profile %s has no saved account information; run `cswitch add` again while logged in to it",
		ZH: "profile %s 没有保存账户信息，请在登录该账户时重新运行 `cswitch add`",
		JA: "プロファイル %s にはアカウント情報が保存されていません。そのアカウントでログイン中に `cswitch add` を再実行してください",
	},
	"err.profile_other_account": {
		EN: "profile %s belongs to %s, but Claude Code is logged in as %s",
		ZH: "profile %s 属于 %s，但 Claude Code 当前登录的是 %s",
		JA: "プロファイル %s は %s のものですが、Claude Code は %s でログインしています",
	},
	"err.backup": {EN: "backup %s: %w", ZH: "备份 %s 失败：%w", JA: "%s のバックアップに失敗しました: %w"},
	"err.secret_corrupt": {
		EN: "the saved %[2]s login of profile %[1]s cannot be decrypted; save it again with `cswitch add`",
		ZH: "profile %[1]s 保存的 %[2]s 登录无法解密，请用 `cswitch add` 重新保存",
		JA: "プロファイル %[1]s に保存された %[2]s のログインを復号できません。`cswitch add` で保存し直してください",
	},
	"err.keychain_locked": {
		EN: "the macOS keychain is locked (this happens over SSH). Unlock it, then retry:\n  security unlock-keychain ~/Library/Keychains/login.keychain-db",
		ZH: "macOS 钥匙串处于锁定状态（通过 SSH 登录时常见）。请先解锁再重试：\n  security unlock-keychain ~/Library/Keychains/login.keychain-db",
		JA: "macOS のキーチェーンがロックされています（SSH 経由のログインでよく起こります）。ロックを解除してから再試行してください:\n  security unlock-keychain ~/Library/Keychains/login.keychain-db",
	},
	"err.keychain_verify": {
		EN: "could not verify the keychain write: %w",
		ZH: "无法确认钥匙串写入成功：%w",
		JA: "キーチェーンへの書き込みを確認できませんでした: %w",
	},
	"err.rollback_failed": {
		EN: "%v; restoring the previous state also failed: %v (backups are in ~/.config/cswitch/backups)",
		ZH: "%v；恢复到之前的状态也失败了：%v（备份位于 ~/.config/cswitch/backups）",
		JA: "%v。元の状態への復元にも失敗しました: %v（バックアップは ~/.config/cswitch/backups にあります）",
	},

	"err.code_not_logged_in": {
		EN: "Claude Code is not logged in",
		ZH: "Claude Code 未登录",
		JA: "Claude Code はログインしていません",
	},
	"err.code_not_logged_in_add": {
		EN: "Claude Code is not logged in; run `claude auth login`, or use --desktop-only",
		ZH: "Claude Code 未登录；请先运行 `claude auth login`，或使用 --desktop-only",
		JA: "Claude Code はログインしていません。`claude auth login` を実行するか、--desktop-only を使ってください",
	},
	"err.code_not_logged_in_add_nodesktop": {
		EN: "Claude Code is not logged in; run `claude auth login` first",
		ZH: "Claude Code 未登录；请先运行 `claude auth login`",
		JA: "Claude Code はログインしていません。先に `claude auth login` を実行してください",
	},
	"err.code_creds_parse": {
		EN: "cannot read Claude Code credentials: %w",
		ZH: "无法读取 Claude Code 凭据：%w",
		JA: "Claude Code の認証情報を読み取れません: %w",
	},
	"err.code_creds_no_oauth": {
		EN: "Claude Code is not logged in with a Claude subscription (API-key logins cannot be switched)",
		ZH: "Claude Code 不是用 Claude 订阅账户登录的（使用 API key 登录的无法切换）",
		JA: "Claude Code は Claude サブスクリプションでログインしていません（API キーでのログインは切り替えられません）",
	},
	"err.code_no_oauth_account": {
		EN: "~/.claude.json has no account information (is Claude Code logged in?)",
		ZH: "~/.claude.json 中没有账户信息（Claude Code 登录了吗？）",
		JA: "~/.claude.json にアカウント情報がありません（Claude Code にログインしていますか？）",
	},
	"err.code_oauth_account_parse": {
		EN: "cannot read the account in ~/.claude.json: %w",
		ZH: "无法读取 ~/.claude.json 中的账户信息：%w",
		JA: "~/.claude.json のアカウント情報を読み取れません: %w",
	},
	"err.code_failed_after_desktop": {
		EN: "Claude Desktop was switched to %s, but switching Claude Code failed: %w",
		ZH: "Claude Desktop 已切换到 %s，但 Claude Code 切换失败：%w",
		JA: "Claude Desktop は %s に切り替わりましたが、Claude Code の切り替えに失敗しました: %w",
	},
	"err.claude_login": {
		EN: "claude auth login failed: %w",
		ZH: "claude auth login 执行失败：%w",
		JA: "claude auth login に失敗しました: %w",
	},

	"err.desktop_not_logged_in": {
		EN: "Claude Desktop is not logged in",
		ZH: "Claude Desktop 未登录",
		JA: "Claude Desktop はログインしていません",
	},
	"err.desktop_unsupported": {
		EN: "Claude Desktop is not available on this platform; cswitch manages Claude Code only here",
		ZH: "此平台上没有 Claude Desktop，cswitch 在这里只管理 Claude Code",
		JA: "このプラットフォームには Claude Desktop がないため、cswitch は Claude Code のみを管理します",
	},
	"err.desktop_unknown_account": {
		EN: "Claude Desktop has a login, but its account cannot be identified (no lastKnownAccountUuid); open Desktop once and try again",
		ZH: "Claude Desktop 有登录信息，但无法识别是哪个账户（缺少 lastKnownAccountUuid）；请先打开一次 Desktop 再重试",
		JA: "Claude Desktop にログイン情報はありますが、アカウントを特定できません（lastKnownAccountUuid がありません）。Desktop を一度起動してから再試行してください",
	},
	"err.desktop_other_account": {
		EN: "Claude Desktop is logged in as a different account (%s) than profile %s",
		ZH: "Claude Desktop 当前登录的账户（%s）与 profile %s 不是同一个",
		JA: "Claude Desktop はプロファイル %[2]s とは別のアカウント（%[1]s）でログインしています",
	},
	"err.bad_desktop_snapshot": {
		EN: "profile %s has a damaged Claude Desktop login: %w",
		ZH: "profile %s 保存的 Claude Desktop 登录已损坏：%w",
		JA: "プロファイル %s の Claude Desktop ログインが破損しています: %w",
	},
	"err.desktop_quit": {
		EN: "could not quit Claude Desktop: %v: %s",
		ZH: "无法退出 Claude Desktop：%v：%s",
		JA: "Claude Desktop を終了できませんでした: %v: %s",
	},
	"err.desktop_quit_timeout": {
		EN: "Claude Desktop did not quit within %s; quit it manually and retry",
		ZH: "Claude Desktop 在 %s 内没有退出；请手动退出后重试",
		JA: "Claude Desktop が %s 以内に終了しませんでした。手動で終了してから再試行してください",
	},
	"err.desktop_open": {
		EN: "could not open Claude Desktop: %v: %s",
		ZH: "无法打开 Claude Desktop：%v：%s",
		JA: "Claude Desktop を起動できませんでした: %v: %s",
	},

	"err.usage_needs_code": {
		EN: "no Claude Code login saved (usage needs one)",
		ZH: "没有保存 Claude Code 登录（查询额度需要它）",
		JA: "Claude Code のログインが保存されていません（使用量の確認に必要です）",
	},
	"err.refresh": {
		EN: "could not refresh the token: %w",
		ZH: "刷新 token 失败：%w",
		JA: "トークンを更新できませんでした: %w",
	},
	"err.fetch_usage": {
		EN: "could not fetch usage: %w",
		ZH: "查询额度失败：%w",
		JA: "使用量を取得できませんでした: %w",
	},

	// ---- v0.2: simpler interaction ----
	"cmd.root.long": {
		EN: `cswitch keeps the logins of several Claude accounts and switches Claude Code
and Claude Desktop between them with one command.

  cswitch                  list accounts with their usage and pick one
  cswitch <name>           switch to an account
  cswitch login            log in to another account (Claude Code)
  cswitch login --desktop  ... and Claude Desktop too
  cswitch usage            usage of every account
  cswitch lang zh          set the language (remembered)

The accounts you are logged in with are saved automatically, named after
their email; rename them with "cswitch rename".`,
		ZH: `cswitch 保存多个 Claude 账户的登录状态，一条命令即可让 Claude Code 和
Claude Desktop 同时切换账户。

  cswitch                  列出所有账户及额度，选择一个切换
  cswitch <名称>           切换到指定账户
  cswitch login            登录另一个账户（Claude Code）
  cswitch login --desktop  同时登录 Claude Desktop
  cswitch usage            查看所有账户的额度
  cswitch lang zh          设置界面语言（会记住）

当前登录的账户会自动保存，名称取自邮箱；可以用 "cswitch rename" 改名。`,
		JA: `cswitch は複数の Claude アカウントのログイン状態を保存し、コマンド一つで
Claude Code と Claude Desktop のアカウントを切り替えます。

  cswitch                  アカウントと使用量を一覧表示して選択
  cswitch <名前>           アカウントを切り替え
  cswitch login            別のアカウントにログイン（Claude Code）
  cswitch login --desktop  Claude Desktop にもログイン
  cswitch usage            全アカウントの使用量
  cswitch lang ja          表示言語を設定（保存されます）

ログイン中のアカウントはメールアドレスから名前を付けて自動保存されます。
名前は "cswitch rename" で変更できます。`,
	},
	"cmd.add.short": {
		EN: "Save the accounts you are logged in with now (name optional)",
		ZH: "保存当前登录的账户（名称可省略）",
		JA: "現在ログイン中のアカウントを保存（名前は省略可）",
	},
	"cmd.add.long": {
		EN: `Save the accounts Claude Code and Claude Desktop are logged in with. Without a
name, each is named after its email (or "desktop-<id>" for a Desktop-only
account). With a name, the Claude Code account gets that name.

You rarely need this: cswitch saves the current logins by itself whenever it
switches.`,
		ZH: `保存 Claude Code 和 Claude Desktop 当前登录的账户。不指定名称时，名称取自邮箱
（只有 Desktop 登录的账户命名为 "desktop-<id>"）；指定名称时，该名称给 Claude
Code 的账户。

一般用不到这个命令：每次切换时 cswitch 都会自动保存当前的登录状态。`,
		JA: `Claude Code と Claude Desktop でログイン中のアカウントを保存します。名前を省略すると
メールアドレスから名前を付けます（Desktop のみのアカウントは "desktop-<id>"）。
名前を指定すると、Claude Code のアカウントにその名前を付けます。

通常は不要です。切り替えのたびに cswitch が現在のログイン状態を自動で保存します。`,
	},
	"cmd.login.short": {
		EN: "Log in to another account and save it",
		ZH: "登录另一个账户并保存",
		JA: "別のアカウントにログインして保存",
	},
	"cmd.login.long": {
		EN: `Log in to another account without logging out of the current one. Logging out
would revoke the saved tokens, so never use the apps' own "Log out" for this.
The current login is saved first.

By default this logs in Claude Code (it runs "claude auth login"). Add --desktop
to also sign in Claude Desktop, or use --desktop-only. For Desktop, cswitch
quits it, clears its login locally, reopens it, and waits for you to sign in.`,
		ZH: `在不登出当前账户的情况下登录另一个账户。登出会吊销已保存的 token，所以不要用
应用自带的「登出」来添加账户。开始前会先保存当前的登录状态。

默认只登录 Claude Code（运行 "claude auth login"）。加 --desktop 会同时登录
Claude Desktop，--desktop-only 只登录 Desktop。登录 Desktop 时，cswitch 会退出
Desktop、在本地清除登录状态后重新打开，等待你登录。`,
		JA: `現在のアカウントからログアウトせずに別のアカウントにログインします。ログアウトすると
保存済みのトークンが無効になるため、アプリの「ログアウト」は使わないでください。
最初に現在のログイン状態を保存します。

既定では Claude Code にログインします（"claude auth login" を実行）。--desktop で
Claude Desktop にも、--desktop-only で Desktop のみにログインします。Desktop の場合、
cswitch は Desktop を終了し、ローカルのログイン状態を消去して再起動し、サインインを
待ちます。`,
	},
	"cmd.use.long": {
		EN: `Switch Claude Code and Claude Desktop to <name>. "cswitch <name>" does the same.

The current logins are saved first (new accounts get an automatic name), since
both apps rotate their tokens. Claude Desktop is quit and reopened; if anything
fails, its previous state is restored. Running claude sessions keep the old
account until restarted.`,
		ZH: `把 Claude Code 和 Claude Desktop 切换到 <名称>。"cswitch <名称>" 效果相同。

两个应用都会轮换 token，所以切换前会先保存当前的登录状态（新账户会自动命名）。
Claude Desktop 会被退出并重新打开；中途出错时会恢复到切换前的状态。正在运行的
claude 会话在重启前仍使用原来的账户。`,
		JA: `Claude Code と Claude Desktop を <名前> に切り替えます。"cswitch <名前>" でも同じです。

どちらのアプリもトークンをローテーションするため、切り替え前に現在のログイン状態を
保存します（新しいアカウントには自動で名前を付けます）。Claude Desktop は終了して
再起動され、途中で失敗した場合は元の状態に戻します。実行中の claude セッションは
再起動するまで元のアカウントのままです。`,
	},
	"cmd.usage.long": {
		EN: `Show the 5-hour and weekly usage limits of each profile, using the same endpoint
as Claude Code's /usage. Profiles are queried in parallel, and expired tokens are
refreshed and saved, just as Claude Code itself does.`,
		ZH: `查看每个账户的 5 小时和每周额度，使用的是 Claude Code /usage 背后的同一个接口。
所有账户并发查询，过期的 token 会像 Claude Code 自己那样刷新并保存。`,
		JA: `各プロファイルの 5 時間と週ごとの使用量上限を表示します。Claude Code の /usage と
同じエンドポイントを使います。プロファイルは並列に照会され、期限切れのトークンは
Claude Code 自身と同じように更新・保存されます。`,
	},
	"cmd.list.short": {
		EN: "List saved accounts",
		ZH: "列出已保存的账户",
		JA: "保存済みアカウントの一覧",
	},
	"cmd.rename.short": {
		EN: "Rename a saved account",
		ZH: "给已保存的账户改名",
		JA: "保存済みアカウントの名前を変更",
	},
	"cmd.lang.short": {
		EN: "Show or set the language (en, zh, ja, auto); the choice is remembered",
		ZH: "查看或设置界面语言（en、zh、ja、auto），设置后会一直使用",
		JA: "表示言語を確認・設定（en、zh、ja、auto）。設定は保存されます",
	},
	"flag.force_use": {
		EN: "switch even if Claude Desktop's current login cannot be saved (it will be lost)",
		ZH: "即使 Claude Desktop 当前的登录无法保存也强制切换（该登录会丢失）",
		JA: "Claude Desktop の現在のログインを保存できなくても切り替える（そのログインは失われます）",
	},
	"flag.login_desktop": {
		EN: "also sign in Claude Desktop",
		ZH: "同时登录 Claude Desktop",
		JA: "Claude Desktop にもサインインする",
	},
	"flag.login_desktop_only": {
		EN: "only sign in Claude Desktop",
		ZH: "只登录 Claude Desktop",
		JA: "Claude Desktop のみにサインインする",
	},
	"flag.usage_all": {
		EN: "also show internal usage entries",
		ZH: "同时显示内部的额度条目",
		JA: "内部の使用量項目も表示する",
	},
	"list.header": {
		EN: "NAME\tEMAIL\tPLAN\tCODE\tDESKTOP",
		ZH: "名称\t邮箱\t套餐\tCODE\tDESKTOP",
		JA: "名前\tメール\tプラン\tCODE\tDESKTOP",
	},
	"list.in_use":       {EN: "in use", ZH: "使用中", JA: "使用中"},
	"list.desktop_only": {EN: "(Desktop only)", ZH: "（仅 Desktop）", JA: "（Desktop のみ）"},
	"pick.header": {
		EN: "#\tNAME\tEMAIL\tPLAN\t5H\t7D\tIN USE",
		ZH: "#\t名称\t邮箱\t套餐\t5小时\t7天\t使用中",
		JA: "#\t名前\tメール\tプラン\t5時間\t7日\t使用中",
	},
	"pick.prompt": {
		EN: "Switch to (number or name, Enter to cancel): ",
		ZH: "切换到（输入编号或名称，直接回车取消）：",
		JA: "切り替え先（番号または名前、Enter でキャンセル）: ",
	},
	"err.bad_choice": {
		EN: "%q is not one of the listed accounts",
		ZH: "%q 不是列表中的账户",
		JA: "%q は一覧にあるアカウントではありません",
	},
	"err.no_such_account": {
		EN: "there is no saved account named %q",
		ZH: "没有名为 %q 的账户",
		JA: "%q という名前の保存済みアカウントはありません",
	},
	"hint.accounts": {
		EN: "saved accounts: %s",
		ZH: "已保存的账户：%s",
		JA: "保存済みアカウント: %s",
	},
	"hint.did_you_mean": {
		EN: "did you mean the command \"cswitch %s\"?",
		ZH: "你是不是想运行 \"cswitch %s\"？",
		JA: "コマンド \"cswitch %s\" のことですか？",
	},
	"err.name_taken": {
		EN: "the name %q is already used by another account (%s)",
		ZH: "名称 %q 已经被另一个账户（%s）使用",
		JA: "名前 %q はすでに別のアカウント（%s）で使われています",
	},
	"err.profile_exists": {
		EN: "an account named %q already exists",
		ZH: "已经有名为 %q 的账户",
		JA: "%q という名前のアカウントはすでに存在します",
	},
	"err.desktop_unknown_account_switch": {
		EN: "Claude Desktop has a login whose account cannot be identified, so it cannot be saved; open Desktop once and retry, or pass --force to discard it",
		ZH: "Claude Desktop 当前的登录无法识别是哪个账户，所以无法保存；请先打开一次 Desktop 再重试，或加 --force 放弃它",
		JA: "Claude Desktop のログインのアカウントを特定できないため保存できません。Desktop を一度起動してから再試行するか、--force で破棄してください",
	},
	"warn.desktop_unknown_lost": {
		EN: "Claude Desktop's current login could not be identified and is discarded",
		ZH: "无法识别 Claude Desktop 当前的登录，已放弃保存",
		JA: "Claude Desktop の現在のログインを特定できないため破棄します",
	},
	"info.auto_saved_code": {
		EN: "Saved the current Claude Code account %s as %q",
		ZH: "已自动保存当前的 Claude Code 账户 %s，名称为 %q",
		JA: "現在の Claude Code アカウント %s を %q として自動保存しました",
	},
	"info.auto_saved_desktop": {
		EN: "Saved the current Claude Desktop account as %q (rename it with: cswitch rename %[1]s <name>)",
		ZH: "已自动保存当前的 Claude Desktop 账户，名称为 %q（可以用 cswitch rename %[1]s <名称> 改名）",
		JA: "現在の Claude Desktop アカウントを %q として自動保存しました（名前の変更: cswitch rename %[1]s <名前>）",
	},
	"info.renamed": {
		EN: "Renamed %s to %s",
		ZH: "已将 %s 改名为 %s",
		JA: "%s を %s に名前変更しました",
	},
	"info.lang_current": {
		EN: "language setting: %s (in use: %s)",
		ZH: "语言设置：%s（当前使用：%s）",
		JA: "言語設定: %s（使用中: %s）",
	},
	"info.lang_saved": {
		EN: "Language set to %s; it will be used from now on",
		ZH: "界面语言已设为 %s，之后都会使用这个语言",
		JA: "表示言語を %s に設定しました。今後はこの言語を使います",
	},
	"info.lang_auto": {
		EN: "Language will follow the system again",
		ZH: "界面语言已恢复为跟随系统",
		JA: "表示言語をシステムに合わせる設定に戻しました",
	},

	// ---- v0.2: verified accounts, limits ----
	"err.fetch_profile": {
		EN: "could not look up the account of the token: %w",
		ZH: "无法查询 token 所属的账户：%w",
		JA: "トークンのアカウントを照会できませんでした: %w",
	},
	"warn.unverified_account": {
		EN: "could not verify with the server which account Claude Code is logged in as (%v); using ~/.claude.json for now",
		ZH: "无法向服务器确认 Claude Code 登录的是哪个账户（%v），暂时以 ~/.claude.json 为准",
		JA: "Claude Code のログインアカウントをサーバーで確認できませんでした（%v）。いったん ~/.claude.json を使います",
	},
	"err.code_login_mislabeled": {
		EN: "the Claude Code login saved in %q actually belongs to %s; it was moved to %q. Log in to this account again with: cswitch login",
		ZH: "%q 里保存的 Claude Code 登录其实属于 %s，已移到 %q。请运行 cswitch login，用这个账户重新登录",
		JA: "%q に保存されていた Claude Code のログインは実際には %s のもので、%q に移しました。cswitch login でこのアカウントに再ログインしてください",
	},
	"err.nothing_to_save": {
		EN: "neither Claude Code nor Claude Desktop is logged in; nothing was saved",
		ZH: "Claude Code 和 Claude Desktop 都没有登录，没有可保存的内容",
		JA: "Claude Code も Claude Desktop もログインしていないため、何も保存していません",
	},
	"err.reserved_name": {
		EN: "%q is a cswitch command name and cannot be used as an account name",
		ZH: "%q 是 cswitch 的命令名，不能用作账户名",
		JA: "%q は cswitch のコマンド名のため、アカウント名には使えません",
	},
	"info.already_saved_as": {
		EN: "%s is already saved as %q; keeping that name",
		ZH: "%s 已经保存为 %q，保留原来的名称",
		JA: "%s はすでに %q として保存されているため、その名前のままにします",
	},
	"usage.label.session":      {EN: "Session", ZH: "当前会话", JA: "現在のセッション"},
	"usage.label.weekly":       {EN: "Week", ZH: "本周", JA: "今週"},
	"usage.label.weekly_model": {EN: "Week (%s)", ZH: "本周 %s", JA: "今週の%s"},
	"usage.not_started":        {EN: "not started", ZH: "未开始", JA: "未開始"},
	"warn.lang_env_overrides": {
		EN: "CSWITCH_LANG=%s is set in your environment and takes precedence over this setting",
		ZH: "环境变量 CSWITCH_LANG=%s 优先于这个设置，需要先去掉它才会生效",
		JA: "環境変数 CSWITCH_LANG=%s がこの設定より優先されます",
	},
}
