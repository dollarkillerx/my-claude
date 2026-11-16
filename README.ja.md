# cswitch

[English](README.md) | [简体中文](README.zh-CN.md) | **日本語**

複数の Claude アカウントをコマンド一つで切り替えます。macOS では **Claude Code** と **Claude Desktop** の両方に同時に反映され、Linux では Claude Code を切り替えます。

```console
$ cswitch
#  名前      メール                プラン  5時間  7日  使用中
1  personal  me@example.com        pro     21%    48%
2  work      me@company.com        max     4%     13%  * Code+Desktop
切り替え先（番号または名前、Enter でキャンセル）: 1
Claude Desktop を終了しています…
Claude Desktop を起動しています…
Claude Desktop → personal
Claude Code → personal（me@example.com）

$ cswitch work          # 名前で直接切り替えることもできます
```

> **非公式ツールです。** cswitch はコミュニティによるツールで、Anthropic とは関係がなく、承認も受けていません。アプリのローカルストレージを直接扱うため、その形式はどのリリースでも変わる可能性があります。[制限事項](#制限事項)を参照してください。

## 機能

- **複数アカウントをコマンド一つで。** アカウントはいくつでも保存でき、`cswitch use <名前>` で両方のアプリを切り替えます。
- **Claude Code と Claude Desktop を同時に。** `--code-only` / `--desktop-only` で片方だけ切り替えることもでき、2 つのアプリが同じアカウントである必要はありません。
- **使用量をひと目で。** `cswitch usage` は全アカウントの 5 時間・週ごとの上限を並列に取得します。Claude Code の `/usage` と同じエンドポイントを使います。
- **Desktop の履歴を共有。** 切り替えのたびに Claude Desktop のローカルセッション（Code タブと Cowork）を全アカウント間で統合するので、どのアカウントでもすべてのセッションが見えます。
- **安全設計。**
  - 保存したログイン状態は macOS のキーチェーンに置かれます。
  - ファイルは変更前に必ずバックアップされ、切り替えに失敗するとロールバックされます。
  - 現在のログインは切り替え前に自動で保存されるため、失われることはありません。
- **English・简体中文・日本語に対応。** システムの言語に合わせて表示され、`--lang` で指定もできます。

## 対応プラットフォーム

| | Claude Code | Claude Desktop |
| --- | --- | --- |
| macOS（Apple Silicon / Intel） | ✅ | ✅ |
| Linux | ✅ | —（公式の Linux 版 Claude Desktop はありません） |

Claude Code は Claude サブスクリプション（Pro / Max / Team / Enterprise）でログインしている必要があります。API キーでのログインは切り替えられません。

## インストール

[リリースページ](https://github.com/dollarkillerx/my-claude/releases)からバイナリをダウンロードします（macOS arm64、Linux x86-64）:

```bash
tar -xzf cswitch_*_darwin_arm64.tar.gz
```

```bash
sudo mv cswitch_*_darwin_arm64/cswitch /usr/local/bin/
```

macOS で Gatekeeper にブロックされた場合は、隔離属性を外してください:

```bash
xattr -d com.apple.quarantine /usr/local/bin/cswitch
```

Go 1.24 以上でソースからビルドすることもできます:

```bash
go install github.com/dollarkillerx/my-claude/cmd/cswitch@latest
```

## はじめに

```bash
cswitch
```

これだけです。引数なしで `cswitch` を実行すると、アカウントと使用量が一覧表示され、番号を入力して切り替えられます。ログイン中のアカウントはメールアドレスから名前を付けて自動保存されます（`alex@example.com` は `alex`）。Desktop のみのアカウントはメールアドレスが取得できないため `desktop-<id>` として保存されます。`cswitch rename desktop-1a2b3c4d work` で名前を付けられます。

### 別のアカウントを追加する

> ⚠️ **アカウントの追加にアプリの「ログアウト」は使わないでください。** ログアウトするとサーバー側でトークンが無効になり、保存したログインも使えなくなります。

```bash
cswitch login
```

現在のログインを保存してから `claude auth login` を実行し、別のアカウントにログインできるようにします。完了すると両方のアカウントが保存されています。

- `cswitch login --desktop`: 新しいアカウントで Claude Desktop にもサインインします。cswitch は Desktop を終了し、ローカルのログイン状態だけを消去（サーバー側ではログアウトしません）して再起動し、サインインを待ちます。
- `cswitch login --desktop-only`: Desktop のみ。
- 名前を自分で付けるには、`cswitch login work` のように指定します。

### 切り替える

```bash
cswitch work
```

`cswitch <名前>` で両方のアプリを切り替えます。`--code-only` / `--desktop-only` で片方だけ切り替えます。切り替えのたびに、まず現在のログイン状態を保存します。実行中の `claude` セッションは約 30 秒以内に新しいアカウントに切り替わります（Claude Code がログイン情報を読み直します）。すぐに反映するには再起動してください。

### 言語を設定する

```bash
cswitch lang ja
```

設定は保存され、以降の実行でも使われます。`cswitch lang auto` でシステムに合わせる設定に戻ります。

### 上限のリセット

アカウントによっては**上限のリセット**が付与されます。週間の使用量やモデル別の利用枠を 1 回だけ無料で戻せるものです（アプリの使用量ページにある「無料でリセット」）。

```bash
cswitch resets            # 各アカウントのリセット: 残り回数、有効期限、今使えるか
cswitch reset work        # 1 回使う（Claude Code が次に使うもの）。実行前に確認します
cswitch reset work <id>   # 指定したものを使う
```

使ったリセットは戻せないため、cswitch が自動で使うことはありません。`cswitch usage` は、リセットが使えるアカウントを表示します。サーバーが Claude Code が古いと答えたとき（`cli_version`）は、`resets` と `reset` が `claude update` を実行してから再確認します。`--no-update` で無効にできます。

### 自動切り替え

```bash
cswitch auto                          # このターミナルで監視
cswitch auto --threshold 80           # 80% で切り替え（既定 90%）
cswitch auto --weekly-threshold 95    # 週間は 95% で切り替え（既定 99%）
cswitch auto --model Fable            # Fable の週間上限も考慮
cswitch auto --install                # バックグラウンドで実行（ログイン時に起動）
cswitch auto --status                 # 実行中か、最後の切り替え
cswitch auto --uninstall              # 停止して削除
```

使用中のアカウントのセッションまたは週間の使用量がしきい値に達すると、余裕が最も大きいアカウントに Claude Code を切り替えます。切り替えの間隔は最低 5 分で、余裕のあるアカウントがないときはログに記録するだけです。Claude Desktop は再起動が必要なため、切り替えるのは Claude Code のみです。バックグラウンドサービスは macOS では launchd の LaunchAgent、Linux では systemd ユーザーサービスで、ログは `~/.config/cswitch/auto.log` で、2 MB 以内に保たれます（それ以前の分は `auto.log.1`）。`--once` は 1 回だけチェック（cron 用）、`--dry-run` は実行内容をログに出すだけです。

### 複数アカウントを同時に、ディレクトリごとに

```bash
cswitch run work                 # このターミナルだけで "work" として Claude Code を起動
cswitch run work --resume        # 名前より後の引数は claude にそのまま渡す
cswitch map work ~/work          # ディレクトリ（とその下すべて）を割り当て
cd ~/work/project && cswitch run # → "work" として起動
cswitch map                      # 割り当て一覧。cswitch unmap ~/work で解除
```

`cswitch run` は Claude Code 自身の `CLAUDE_CONFIG_DIR` を使います。アカウントごとに `~/.config/cswitch/sessions/` にセッションディレクトリを持つため、他のターミナルやグローバルのログインには影響せず、複数のアカウントを並行して使えます。セッションは `~/.claude` の設定・CLAUDE.md・skills・プラグイン・会話履歴を共有します（`--resume` はアカウントをまたいで使えます）。セッションが自分で更新したトークンは、終了時にアカウントへ保存し直します。先にターミナルを閉じた場合も、次に cswitch がそのアカウントを使うときに保存します。セッションの実行中は、同じログインの複製が互いを無効にしてしまうため、グローバルのログインをそのアカウントに切り替えません。割り当てのないディレクトリや、すでにグローバルのログインになっているアカウントを指定した場合は、そのまま `claude` を起動します。

### 使用量の取得とレート制限

使用量エンドポイント自体にレート制限があるため、取得結果はキャッシュされます。使用中のアカウント（Claude Code のアカウント、または `cswitch run` のセッションが動いているアカウント）は 1 分ごと、その他は 5 分ごとに取得します。429 が返ったときはサーバーの `Retry-After`（なければ 1 分から最大 30 分まで段階的に延長）だけ待ち、その間は前回のデータを何分前のものか付けて表示します。レート制限の表示は、そのデータが 15 分より古くなったときだけです。`cswitch usage --refresh` で今すぐ取得します。

## コマンド一覧

| コマンド | 説明 |
| --- | --- |
| `cswitch` | アカウントと使用量を一覧表示し、選んで切り替え |
| `cswitch <名前>` | アカウントを切り替え（`cswitch use <名前>` と同じ） |
| `cswitch login [名前]` | ログアウトせずに別のアカウントにログイン。`--desktop`、`--desktop-only` |
| `cswitch usage [名前...]` | 使用量を表示。`--refresh` で今すぐ取得、`--all` で内部の項目も表示 |
| `cswitch resets [名前...]` | 上限のリセットを表示 |
| `cswitch reset <名前> [id]` | 上限のリセットを 1 回使う（確認あり、`-y` で省略） |
| `cswitch auto` | 上限が近づいたら自動で切り替え。`--install`、`--status`、`--uninstall`、`--once`、`--dry-run` |
| `cswitch run [名前] [引数]` | このターミナルだけで指定アカウントの Claude Code を起動 |
| `cswitch map [名前] [ディレクトリ]` / `unmap [ディレクトリ]` | `run` 用にディレクトリをアカウントに割り当て、一覧表示、解除 |
| `cswitch list` | 保存済みアカウントの一覧（エイリアス `ls`） |
| `cswitch current` | 各アプリが使用中のアカウントを表示 |
| `cswitch add [名前]` | 現在のログインを今すぐ保存（通常は不要。切り替え時に自動保存されます） |
| `cswitch rename <旧名> <新名>` | アカウントの名前を変更（エイリアス `mv`） |
| `cswitch remove <名前>` | 保存済みアカウントを削除（エイリアス `rm`）。ログアウトはしません。`-y` |
| `cswitch lang [en\|zh\|ja\|auto]` | 表示言語の確認・設定。設定は保存されます |
| `cswitch sync` | Desktop のローカルセッションを今すぐ全アカウントで共有（切り替え時にも自動実行） |

切り替えのオプション（`cswitch <名前>` と `use`）: `--code-only`、`--desktop-only`、`--no-launch`、`--force`。`--force` は、アカウントを特定できない Desktop のログインを破棄します（通常は上書きを拒否します）。グローバルオプション: `--lang`、`--version`。

## 言語

表示言語は次の順で決まります:

1. `--lang`（その実行のみ）
2. `CSWITCH_LANG`
3. `cswitch lang` で保存した言語
4. `LC_ALL`、`LC_MESSAGES`、`LANG`
5. macOS のシステム言語

## 履歴の共有: 共有できるもの・できないもの

| | 共有 |
| --- | --- |
| Claude Desktop のローカルセッション: Code タブと Cowork（`claude-code-sessions`、`local-agent-mode-sessions`） | ✅ 切り替えのたびに統合 |
| Claude Code の会話記録（`~/.claude/projects`） | ✅ もともとアカウント共通 |
| claude.ai のチャット、Projects、Artifacts | ❌ アカウントごとに Anthropic のサーバーに保存 |

同じセッションが複数のアカウントにある場合は、最新のコピーが優先されます。あるアカウントで削除したセッションはコピーし直されず、`~/.config/cswitch/backups/sessions-trash/` に移動されます。

## 仕組み

概要:

- **Claude Code**: キーチェーンの `Claude Code-credentials` と、`~/.claude.json` の `oauthAccount`。
- **Claude Desktop**: `config.json` 内の暗号化された `oauth:*` トークン、Cookie データベース、claude.ai の Web ストレージ。これらはバイト単位でそのままコピーされ、復号されることはありません。
- **保存済みプロファイル**: ログイン情報は AES-256-GCM で暗号化したファイルとして保存され、その鍵はキーチェーン（サービス名 `cswitch`）に置かれます。メタデータは `~/.config/cswitch`（パーミッション 0700）に保存されます。
- **切り替えのたびに**: 両アプリともリフレッシュトークンをローテーションするため、まず現在のログイン状態をそれぞれのプロファイルへ保存し直します。

詳しくは [docs/how-it-works.md](docs/how-it-works.md)（英語）を参照してください。

## データとバックアップ

```
~/.config/cswitch/
├── profiles/<名前>/meta.json      # アカウント情報（シークレットは含まない）
├── profiles/<名前>/*.enc          # 暗号化されたログイン情報（鍵はキーチェーン）
├── profiles/<名前>/desktop/       # このアカウントの Desktop の Cookie と Web ストレージ
├── backups/                       # cswitch が変更する前のファイルのコピー（各ファイル最新 10 件）
│   └── sessions-trash/            # あるアカウントで削除された Desktop セッション
├── sessions/<名前>/               # cswitch run: アカウントごとの CLAUDE_CONFIG_DIR
├── mappings.json                  # cswitch map: ディレクトリ → アカウント
├── usage-cache.json               # 最近の使用量データ（レート制御）
├── auto.log, auto-state.json      # cswitch auto: ログ（と auto.log.1）と最後の切り替え
└── sync-state.json                # 前回のセッション同期時のファイル一覧
```

手動で元に戻すには、両方のアプリを終了してから、必要なファイルを `backups/` から元の場所にコピーします。

アンインストールするには、各プロファイルについて `cswitch remove <名前>` を実行し、`~/.config/cswitch` を削除してから、キーチェーンの `cswitch` 項目を削除します（`security delete-generic-password -s cswitch -a master-key`）。

## 制限事項

- **Linux では Claude Code のみ対応。** Desktop 関連のコマンドやオプションは、Desktop が使えないことを表示します。Linux にはキーチェーンがないため、保存したログイン情報は `~/.config/cswitch/master.key`（パーミッション 0600）の鍵で暗号化されます。これは Linux 上で Claude Code が自身の `~/.claude/.credentials.json` を保護するのと同じ水準です。
- **macOS に SSH で接続して使う場合は、先にキーチェーンのロックを解除してください:** `security unlock-keychain ~/Library/Keychains/login.keychain-db`。ログインキーチェーンは画面からログインしたときにだけ自動で解除されます。`claude` 自体にも同じ制限があります。
- **`CLAUDE_CONFIG_DIR` には対応していません。** cswitch は Claude Code の既定の場所を前提とします。
- **非公開のローカル形式に依存しています。** Claude Code や Claude Desktop のアップデートで動かなくなる可能性があります。認識できない形式を見つけた場合、cswitch は書き込まずにエラーで停止します。
- **実行中の `claude` セッションは再起動するまで元のアカウントのままです。** また、元のアカウントを `~/.claude.json` に書き戻すことがあります。cswitch はキーチェーンのトークンからこれを検出し、次の切り替え時に修正します。
- **使用中のアカウントの使用量:** そのトークンの期限が切れていて `claude` が実行中の場合、cswitch はトークンを更新しません。更新すると実行中のセッションがログアウトされてしまうためです。`claude` を少し使うか終了してから再試行してください。

## コントリビュート

バグ報告、翻訳、プルリクエストを歓迎します。[CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。セキュリティに関する問題は [SECURITY.md](SECURITY.md) を参照してください。

## ライセンス

[MIT](LICENSE)
