# lifestyle-logger

個人の生活ログを、日付単位のJSONへ追記するための小さなCLIです。

このリポジトリには個人のログ、固有のワークスペース設定、認証情報を含めません。実行時に指定したデータディレクトリだけを読み書きします。実装はGo標準ライブラリだけで構成し、外部APIへの通信も行いません。

## データ配置

デフォルトの保存先は次のとおりです。

```text
~/.local/share/lifestyle-logs/YYYY/MM/YYYY-MM-DD.json
```

`--data-dir` または `LIFESTYLE_LOG_DIR` で保存先を変更できます。既存のプライベートなログを扱う場合も、データディレクトリを明示してCLI本体と分離してください。

```bash
export LIFESTYLE_LOG_DIR="$HOME/path/to/private/lifestyle-logs"
```

新規ファイルには、リポジトリ内のスキーマを参照する相対 `$schema` フィールドが入ります。スキーマは [schema/lifestyle-log.schema.json](schema/lifestyle-log.schema.json) にあります。

## 使い方

```bash
go run ./cmd/lifestyle-logger event \
  --date 2026-10-14 \
  --category culture \
  --location "local library" \
  --title "Morning reading" \
  --details "Read for forty minutes before work." \
  --tags "reading,habit" \
  --summary "A calm morning with reading"

go run ./cmd/lifestyle-logger decision \
  --date 2026-10-14 \
  --topic "Weekly planning" \
  --context "Several tasks compete for the same evening." \
  --decision "Assign one main theme to each evening."

go run ./cmd/lifestyle-logger topic \
  --date 2026-10-14 \
  --topic "Conversation review" \
  --takeaways "Keep observations separate from assumptions;Write down the next action"
```

グローバルオプションはコマンド名の前に置きます。

```bash
go run ./cmd/lifestyle-logger \
  --data-dir "$HOME/path/to/private/lifestyle-logs" \
  event --title "Example" --details "Example details"
```

`event`、`decision`、`topic` は、同じ日付のファイルへそれぞれ追記します。書き込みは一時ファイルからのリネームで行い、途中状態のJSONを残さないようにしています。ただし、複数プロセスから同じ日付を同時更新するためのロックは持たないため、同時書き込みは避けてください。

## 開発

依存関係はありません。ローカル検証は次で実行します。

```bash
./scripts/verify-all.sh
```

検証内容は `gofmt`、`go vet`、単体テスト、ビルドです。
