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
	"flag.force_login": {
		EN: "continue even if the current login is not saved in any profile",
		ZH: "即使当前登录的账户没有保存到任何 profile 也继续",
		JA: "現在のログインがどのプロファイルにも保存されていなくても続行する",
	},
	"flag.force_use": {
		EN: "switch even if the current login is not saved in any profile (it will be lost)",
		ZH: "即使当前登录的账户没有保存也强制切换（该登录状态会丢失）",
		JA: "現在のログインが保存されていなくても切り替える（そのログイン状態は失われます）",
	},
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
	"cmd.root.long": {
		EN: `cswitch saves the logins of several Claude accounts and switches Claude Code
and Claude Desktop between them with one command. It also shows each account's
plan usage and shares Claude Desktop's local sessions across all accounts.

Get started:
  cswitch add work                     save the account you are logged in with
  cswitch login personal               log in to another account and save it
  cswitch use personal                 switch both apps
  cswitch usage                        show usage of every account`,
		ZH: `cswitch 保存多个 Claude 账户的登录状态，一条命令即可让 Claude Code 和
Claude Desktop 同时切换账户。它还能查看每个账户的额度，并在所有账户之间共享
Claude Desktop 的本地会话。

快速开始：
  cswitch add work                     保存当前登录的账户
  cswitch login personal               登录另一个账户并保存
  cswitch use personal                 同时切换两个应用
  cswitch usage                        查看所有账户的额度`,
		JA: `cswitch は複数の Claude アカウントのログイン状態を保存し、コマンド一つで
Claude Code と Claude Desktop のアカウントを同時に切り替えます。各アカウントの
使用量の確認や、Claude Desktop のローカルセッションの全アカウント共有もできます。

はじめに:
  cswitch add work                     現在ログイン中のアカウントを保存
  cswitch login personal               別のアカウントにログインして保存
  cswitch use personal                 両方のアプリを切り替え
  cswitch usage                        全アカウントの使用量を表示`,
	},
	"cmd.add.short": {
		EN: "Save the currently logged-in account as a profile (or update it)",
		ZH: "把当前登录的账户保存为 profile（已存在则更新）",
		JA: "現在ログイン中のアカウントをプロファイルとして保存（既存なら更新）",
	},
	"cmd.add.long": {
		EN: `Save the account Claude Code is logged in as under <name>. If Claude Desktop is
logged in as the same account, its login is saved too (Desktop is restarted
briefly so its files are copied consistently).

Desktop is often logged in as a different account than Claude Code; save that
one with --desktop-only under its own name.`,
		ZH: `把 Claude Code 当前登录的账户保存为 <name>。如果 Claude Desktop 登录的是同一个
账户，也会一并保存（Desktop 会短暂重启，以便完整地复制它的文件）。

Desktop 登录的账户经常和 Claude Code 不同，这时用 --desktop-only 把它单独保存为
另一个名字。`,
		JA: `Claude Code でログイン中のアカウントを <name> として保存します。Claude Desktop が
同じアカウントでログインしていれば、そのログインも保存します（ファイルを
一貫した状態でコピーするため、Desktop を一時的に再起動します）。

Desktop が Claude Code と別のアカウントでログインしている場合は、
--desktop-only で別の名前として保存してください。`,
	},
	"cmd.login.short": {
		EN: "Log in to a new account and save it as a profile",
		ZH: "登录一个新账户并保存为 profile",
		JA: "新しいアカウントにログインしてプロファイルとして保存",
	},
	"cmd.login.long": {
		EN: `Log in to another account without logging out of the current one. Logging out
would revoke the saved tokens, so never use the apps' own "Log out" for this.
The current logins are saved back first.

Claude Code:    runs "claude auth login".
Claude Desktop: quits Desktop, clears its login locally, reopens it, and waits
                for you to sign in.`,
		ZH: `在不登出当前账户的情况下登录另一个账户。登出会吊销已保存的 token，所以不要
用应用自带的「登出」来添加账户。开始前会先把当前的登录状态存回对应的 profile。

Claude Code：    运行 "claude auth login"。
Claude Desktop： 退出 Desktop，在本地清除登录状态后重新打开，等待你登录。`,
		JA: `現在のアカウントからログアウトせずに別のアカウントにログインします。
ログアウトすると保存済みのトークンが無効になるため、アプリの「ログアウト」は
使わないでください。最初に現在のログイン状態を保存し直します。

Claude Code:    "claude auth login" を実行します。
Claude Desktop: Desktop を終了し、ローカルのログイン状態を消去して再起動し、
                サインインを待ちます。`,
	},
	"cmd.list.short": {
		EN: "List saved profiles (C = active in Claude Code, D = active in Claude Desktop)",
		ZH: "列出已保存的 profile（C = Claude Code 正在使用，D = Claude Desktop 正在使用）",
		JA: "保存済みプロファイルの一覧（C = Claude Code で使用中、D = Claude Desktop で使用中）",
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
	"cmd.use.long": {
		EN: `Switch Claude Code and Claude Desktop to <name>.

Before switching, the current logins are saved back into their profiles, since
both apps rotate their tokens. Claude Desktop is quit and reopened; if anything
fails, its previous state is restored. Running claude sessions keep the old
account until restarted.`,
		ZH: `把 Claude Code 和 Claude Desktop 切换到 <name>。

两个应用都会轮换 token，所以切换前会先把当前的登录状态存回各自的 profile。
Claude Desktop 会被退出并重新打开；中途出错时会恢复到切换前的状态。
正在运行的 claude 会话在重启前仍使用原来的账户。`,
		JA: `Claude Code と Claude Desktop を <name> に切り替えます。

どちらのアプリもトークンをローテーションするため、切り替え前に現在のログイン
状態をそれぞれのプロファイルへ保存し直します。Claude Desktop は終了して再起動
されます。途中で失敗した場合は元の状態に戻します。実行中の claude セッションは
再起動するまで元のアカウントのままです。`,
	},
	"cmd.usage.short": {
		EN: "Show plan usage for all (or the given) profiles",
		ZH: "查看所有（或指定）profile 的额度使用情况",
		JA: "全（または指定した）プロファイルの使用量を表示",
	},
	"cmd.usage.long": {
		EN: `Show the 5-hour and weekly usage limits of each profile, using the same endpoint
as Claude Code's /usage. Profiles are queried in parallel and expired tokens are
refreshed, except for the active account while a claude process is running.`,
		ZH: `查看每个 profile 的 5 小时和每周额度，使用的是 Claude Code /usage 背后的同一个接口。
所有 profile 并发查询，过期的 token 会自动刷新；但只要有 claude 进程在运行，
就不会刷新当前账户的 token。`,
		JA: `各プロファイルの 5 時間と週ごとの使用量上限を表示します。Claude Code の /usage と
同じエンドポイントを使います。プロファイルは並列に照会され、期限切れのトークンは
更新されます。ただし claude プロセスの実行中は、使用中のアカウントのトークンは
更新しません。`,
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
	"list.header": {
		EN: "ACTIVE\tNAME\tEMAIL\tORG\tPLAN\tSAVED",
		ZH: "使用中\t名称\t邮箱\t组织\t套餐\t已保存",
		JA: "使用中\t名前\tメール\t組織\tプラン\t保存済み",
	},
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
	"info.desktop_other_account": {
		EN: "Note: Claude Desktop is logged in as a different account; save it with `cswitch add <other-name> --desktop-only`",
		ZH: "提示：Claude Desktop 登录的是另一个账户，可以用 `cswitch add <其他名称> --desktop-only` 保存它",
		JA: "注意: Claude Desktop は別のアカウントでログインしています。`cswitch add <別の名前> --desktop-only` で保存できます",
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
	"warn.code_account_repaired": {
		EN: "~/.claude.json named a different account than the Claude Code token; restored the account of profile %s",
		ZH: "~/.claude.json 中的账户与 Claude Code 的 token 不一致，已恢复为 profile %s 的账户",
		JA: "~/.claude.json のアカウントが Claude Code のトークンと一致しないため、プロファイル %s のアカウントに戻しました",
	},
	"warn.code_unsaved_lost": {
		EN: "Claude Code account %s is not saved in any profile; its login will be lost",
		ZH: "Claude Code 账户 %s 没有保存到任何 profile，它的登录状态将会丢失",
		JA: "Claude Code のアカウント %s はどのプロファイルにも保存されていないため、ログイン状態は失われます",
	},
	"warn.desktop_unsaved_lost": {
		EN: "Claude Desktop account %s is not saved in any profile; its login will be lost",
		ZH: "Claude Desktop 账户 %s 没有保存到任何 profile，它的登录状态将会丢失",
		JA: "Claude Desktop のアカウント %s はどのプロファイルにも保存されていないため、ログイン状態は失われます",
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
	"err.account_taken": {
		EN: "this account is already saved as profile %q",
		ZH: "这个账户已经保存为 profile %q",
		JA: "このアカウントはすでにプロファイル %q として保存されています",
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
	"err.code_unsaved": {
		EN: "Claude Code is logged in as %s, which is not saved; run `cswitch add <name>` first, or pass --force to discard it",
		ZH: "Claude Code 当前登录的 %s 还没有保存；请先运行 `cswitch add <名称>`，或加 --force 放弃它",
		JA: "Claude Code でログイン中の %s は保存されていません。先に `cswitch add <名前>` を実行するか、--force で破棄してください",
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
	"err.desktop_unsaved": {
		EN: "Claude Desktop is logged in as account %s, which is not saved; run `cswitch add <name> --desktop-only` first, or pass --force to discard it",
		ZH: "Claude Desktop 当前登录的账户 %s 还没有保存；请先运行 `cswitch add <名称> --desktop-only`，或加 --force 放弃它",
		JA: "Claude Desktop でログイン中のアカウント %s は保存されていません。先に `cswitch add <名前> --desktop-only` を実行するか、--force で破棄してください",
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
	"err.usage_live_expired": {
		EN: "the access token has expired and claude is running; cswitch will not refresh it under a running session (that would log it out). Use claude for a moment or quit it, then retry",
		ZH: "access token 已过期，而 claude 正在运行；为避免把正在运行的会话踢下线，cswitch 不会刷新它。请先用一下 claude 或退出它，再重试",
		JA: "アクセストークンの期限が切れていて、claude が実行中です。実行中のセッションがログアウトされるため、cswitch はトークンを更新しません。claude を少し使うか終了してから再試行してください",
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
}
