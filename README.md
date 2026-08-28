# MiQ

[![CI](https://img.shields.io/github/actions/workflow/status/tikipiya/MiQ/ci.yml?branch=main&label=ci)](https://github.com/tikipiya/MiQ/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/tikipiya/MiQ.svg)](https://pkg.go.dev/github.com/tikipiya/MiQ)
[![License](https://img.shields.io/github/license/tikipiya/MiQ)](LICENSE)

メッセージから引用画像を生成するpure Goライブラリです。ローカル描画、会話画像、Discord・Misskey・X入力、Twemoji、Google Fonts、PNG/JPEG/WebP/AVIF、アセット管理CLI、Voids APIクライアントを提供します。

最低Goバージョンは1.24です。通常の描画とCLIは`CGO_ENABLED=0`でビルドできます。

## Install

```sh
go get github.com/tikipiya/MiQ
go install github.com/tikipiya/MiQ/cmd/miq@latest
```

## Quote image

```go
package main

import (
	"context"
	"os"

	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/theme"
)

func main() {
	engine, err := miq.NewEngine(miq.EngineOptions{})
	if err != nil {
		panic(err)
	}

	img, err := engine.RenderQuote(context.Background(), miq.Quote{
		Text:        "小さな工夫が、毎日の使いやすさをつくる。",
		Username:    "sample_user",
		DisplayName: "Sample User",
		Watermark:   "Make it a Quote",
		Avatar:      miq.ImageFile("avatar.png"),
	}, miq.RenderOptions{Theme: theme.Preset(theme.Dark)})
	if err != nil {
		panic(err)
	}

	file, err := os.Create("quote.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := miq.Encode(file, img, miq.EncodeOptions{Format: miq.PNG}); err != nil {
		panic(err)
	}
}
```

`ImageFile`のほかに`ImageURL`、`ImageBytes`、`ImageValue`を使用できます。remote画像は既定でloopback、private、link-local networkを拒否します。

## コードの書き方

### Engineは再利用する

`Engine`はfont、emoji、remote画像のcacheを共有し、複数goroutineから安全に呼び出せます。画像ごとに作り直さず、applicationの起動時に1つ作成して再利用してください。CLIなどで直接ファイルへ保存するだけなら、描画とencodeをまとめる`WriteQuote`が最短です。

```go
engine, err := miq.NewEngine(miq.EngineOptions{})
if err != nil {
	return err
}

out, err := os.Create("quote.png")
if err != nil {
	return err
}
defer out.Close()

return engine.WriteQuote(
	ctx,
	out,
	miq.Quote{Text: "読みやすい引用画像をGoで生成します。"},
	miq.RenderOptions{},
	miq.EncodeOptions{Format: miq.PNG},
)
```

### 描画結果をbyte列で受け取る

HTTP response、object storage、databaseなどへ渡す場合は、`RenderQuote`の結果を`EncodeBytes`でencodeします。`RenderQuote`は`*image.NRGBA`を返すため、encode前に独自の画像処理を加えることもできます。

```go
img, err := engine.RenderQuote(ctx, miq.Quote{
	Text:        "必要な形式に合わせて出力できます。",
	Username:    "sample_user",
	DisplayName: "Sample User",
}, miq.RenderOptions{Theme: theme.Preset(theme.Light)})
if err != nil {
	return nil, err
}

data, err := miq.EncodeBytes(img, miq.EncodeOptions{
	Format:  miq.WebP,
	Quality: 90,
})
if err != nil {
	return nil, err
}
return data, nil
```

`Encode`は`io.Writer`、`EncodeBytes`は`[]byte`、`EncodeDataURL`は`data:` URLを返します。JPEG、WebP、AVIFの`Quality`は1〜100で、未指定時は92です。

### contextとエラーを扱う

remote avatar、font、emojiの取得は渡された`context.Context`に従います。HTTP handlerでは`r.Context()`、batch処理ではtimeout付きcontextを渡すと、client切断や期限超過時に処理を中断できます。

```go
ctx, cancel := context.WithTimeout(parent, 10*time.Second)
defer cancel()

img, err := engine.RenderQuote(ctx, quote, options)
if err != nil {
	switch {
	case errors.Is(err, miq.ErrValidation):
		// 入力値を修正する
	case errors.Is(err, miq.ErrAsset):
		// avatarやemojiの取得失敗を処理する
	default:
		// font、render、contextなどのエラーを処理する
	}
	return nil, err
}
return img, nil
```

主な分類は`ErrValidation`、`ErrAsset`、`ErrFont`、`ErrRender`、`ErrAPI`です。詳細なfield名が必要な場合は`errors.As`で`*miq.FieldError`を取得できます。

## Themes and output

組み込みテーマは`dark`、`light`、`color`、`portrait`、`portrait-light`です。`theme.Input`でlayout、色、gradient、avatar、text、quote mark、divider、labelを個別に上書きできます。

```go
	width, height := 1280, 720
	options := miq.RenderOptions{
		Theme: theme.Input{
			Extends: theme.Portrait,
			Width:   &width,
			Height:  &height,
	},
	Scale: 2,
}
```

出力形式は`miq.PNG`、`miq.JPEG`、`miq.WebP`、`miq.AVIF`です。`Encode`、`EncodeBytes`、`EncodeDataURL`を利用できます。

## Source adapters

- `adapter/discord`: message、member、mention、Discord Markdown、timestamp
- `adapter/misskey`: note、remote handle、MFM
- `adapter/twitter`: tweet、API v2、FxTwitter
- `markup/commonmark`: 一般的なMarkdownのplain text化

各adapterは外部SDKに依存しない最小構造体を受け取り、`miq.Quote`または`miq.ConversationMessage`へ変換します。

## Conversation

```go
ctx := context.Background()
img, err := engine.RenderConversation(ctx, []miq.ConversationMessage{
	{Username: "sample_user", DisplayName: "Sample User", Text: "最初のメッセージ", Avatar: miq.ImageFile("avatar.png")},
	{Username: "sample_user", DisplayName: "Sample User", Text: "同じ人の連続投稿はまとまります", Avatar: miq.ImageFile("avatar.png")},
	{Username: "another_user", Text: "返信メッセージ"},
}, miq.ConversationOptions{Theme: miq.ConversationDark, Width: 600})
if err != nil {
	return nil, err
}
return miq.EncodeBytes(img, miq.EncodeOptions{Format: miq.PNG})
```

## CLI

```sh
miq generate --text "hello" --username cat --avatar avatar.png --out quote.png
miq install
miq install fonts "Dela Gothic One"
miq ls
miq env
```

主なコマンドは`install`、`uninstall`、`ls`、`search`、`outdated`、`update`、`prune`、`env`、`generate`です。

## Offline assets

Unicode emojiはTwemoji、`<:name:id>`はDiscord CDN、`:name:`と`:name@host:`はMisskeyから取得します。日本語フォントはsystem、disk cache、Google Fontsの順に解決されます。

`miq install`で事前取得すると、`EngineOptions{Offline: true}`でnetworkを使用せずに描画できます。cache先は既定で`.makeitaquote/fonts`と`.makeitaquote/twemoji`です。

## Voids API

Voidsは第三者運営の外部APIです。ローカル描画とは独立した`api/voids`パッケージに分離されています。

```go
client, err := voids.NewClient(voids.Options{})
png, err := client.Direct(ctx, voids.Quote{Text: "hello", Username: "cat"})
hostedURL, err := client.HostedURL(ctx, voids.Quote{Text: "hello"})
```

## Gallery and verification

コミット済みの`docs/visual`を基準に全16グループ・96画像を生成し、形式、寸法、30×16 block RGB平均差を検証します。

```sh
go run ./cmd/miq-gallery --compare --out docs/visual-go
go run ./cmd/miq-gallery --offline --compare --out docs/visual-go-offline
go run ./cmd/miq-gallery --out docs/visual --site docs/index.html
```

既定の知覚差分閾値は`0.08`です。`--offline`ではnetwork依存27件を除く69件を検証します。`--site`はJavaScriptを使わない静的gallery HTMLを生成します。

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 go test -tags nodynamic ./...
CGO_ENABLED=0 go build -tags nodynamic ./...
```

詳細は[CONTRIBUTING.md](CONTRIBUTING.md)、互換性テストは[COMPATIBILITY_TESTS.md](COMPATIBILITY_TESTS.md)を参照してください。

## License and attribution

コードはMIT Licenseです。標準Unicode emoji画像には[Twemoji](https://github.com/jdecked/twemoji)を使用します。公開画像にはTwemojiのCC-BY 4.0 attributionが必要です。詳細は[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)を参照してください。
