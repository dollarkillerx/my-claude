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

`cswitch <名前>` で両方のアプリを切り替えます。`--code-only` / `--desktop-only` で片方だけ切り替えます。切り替えのたびに、まず現在のログイン状態を保存します。切り替え後は実行中の `claude` セッションを再起動してください。再起動するまでは元のアカウントのままです。

### 言語

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
