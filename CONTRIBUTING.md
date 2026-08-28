# Contributing

## Setup

Go 1.24以降が必要です。

```sh
go mod download
go test ./...
```

## Required checks

PRを出す前に次を実行してください。

```sh
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go test -tags nodynamic ./...
CGO_ENABLED=0 go build -tags nodynamic ./...
```

描画結果を変更した場合は全gallery比較も実行します。

```sh
go run ./cmd/miq-gallery --compare --out docs/visual-go
```

意図した見た目の変更では、`docs/visual`を再生成して差分をreviewしてください。

```sh
go run ./cmd/miq-gallery --out docs/visual --site docs/index.html
```

## Layout

- root package: public renderer、types、encoding
- `theme`: preset、color、theme resolution
- `adapter`: Discord、Misskey、X input conversion
- `markup`: CommonMark、Discord Markdown、MFM
- `asset`: font・Twemoji install/cache management
- `api/voids`: external Voids client
- `cmd/miq`: end-user CLI
- `cmd/miq-gallery`: visual gallery generator and comparator
- `internal`: drawing、layout、font、emoji、codec、network policy

外部依存の型を公開APIへ出さず、network pathには`context.Context`と既存のpolicy/cache境界を使用してください。既存のユーザー変更やgolden画像を意図なく更新しないでください。

## Release

release workflowは`v1.2.3`形式のtag push、または同形式のversionを指定したmanual dispatchで実行します。Linux、macOS、Windows向けの`miq`をCGOなしでcross buildし、archiveとSHA-256 checksumをGitHub Releaseへ公開します。
