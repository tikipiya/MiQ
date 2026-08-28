# Go互換性テスト台帳

cutover前の35テストファイル・576ケースを、旧実装の詳細ではなく公開動作の契約単位でGoへ移した台帳です。Goではcompile-timeに拒否される型違反や、Go APIに存在しないbuilder固有のsource互換は対象外です。意味互換・視覚互換・I/O互換は下表のGoテストで固定します。

| 旧実装の領域 | Go側の契約 | 状態 |
| --- | --- | --- |
| `theme/color`, `theme/resolve` | CSS Color 4、整数/配列色、deep partial、preset、validation、copy分離 | `theme/*_test.go`で移植済み |
| `text/breakpoint`, `text/wrap`, `text/fit` | grapheme、BudouX文節優先、禁則、改行、ellipsis/shrink | `internal/emoji/*_test.go`、`internal/textlayout/*_test.go`で移植済み |
| `text/segment`, `emoji/loader`, `emoji/twemojiStore` | Unicode/Discord/Misskey emoji、cache、同時取得集約、negative cache、offline store | `internal/emoji/*_test.go`、`asset/*_test.go`で移植済み |
| `text/markdown`, `discordMarkdown`, `mfm` | CommonMark、Discord記法、MFM除去とmention/timestamp変換 | `markup/*/*_test.go`、`adapter/discord/*_test.go`で移植済み |
| `core/source`, `note`, `tweet`, `mentions` | Discord、Misskey、X/FxTwitterのsource選択、名前/avatar fallback、会話変換 | `adapter/*/*_test.go`で移植済み |
| `core/sizing`, `render/layout`, `render/avatar` | scale、sizeToAvatar、左右/stacked/circle/fit、remote/local/bytes image、cacheとSSRF制約 | `engine_test.go`、`internal/layout/layout_test.go`で移植済み |
| `render/pipeline`, `textStyle` | theme、gradient、background、font weight、divider、quote mark、watermark、全出力形式 | `engine_test.go`、`theme/presets_test.go`で移植済み |
| `render/conversationPipeline`, `core/conversation` | validation、grouping、dark/light、avatar fallback、adapter conversation | `engine_test.go`、`adapter/*/*_test.go`で移植済み |
| `font/catalogue`, `autoload`, `diskCache`, `install`, `registry` | 18フォント、alias、候補提示、Google Fonts、offline/legacy flat cache、atomic cache | `asset/*_test.go`、`internal/font/*_test.go`で移植済み |
| `output/encode` | png/jpeg/jpg/webp/avif、quality、MIME、data URL | `engine_test.go`で移植済み |
| `api/client` | endpoint、snake_case、timeout、retry、bounded body、typed error、SSRF | `api/voids/client_test.go`で移植済み |
| `cli/cli`, `cli/env` | command/alias、exit code、JSON、asset管理、generate flags/format/scale | `cmd/miq/main_test.go`で移植済み |
| `util/assetCache`, `util/projectRoot` | 各loader所有cache、LRU/TTL/in-flight、最寄り`go.mod`のproject-local cache | `engine_test.go`、`internal/emoji/loader_test.go`、`internal/font/manager_test.go`で置換済み |

## Visual oracle

`docs/visual`はGo generatorが生成するcanonical goldenです。`visual_compat_test.go`は代表6ケースを通常の`go test`で比較します。加えて`cmd/miq-gallery`は全16グループ・96ケースの画像形式、寸法、30×16 block RGB平均差を全件検証します。

- dark / light
- portrait
- avatar right
- circular avatar
- English wrapping

dimensionは完全一致を要求します。pixelはrasterizerやsystem fontの差を吸収するため30×16 blockへdownsampleし、RGB平均絶対差0.08以下を要求します。現在の実測は0.009〜0.026です。これによりglyphの微差を許容しつつ、avatar位置、gradient方向、text block、theme背景の崩れを検出します。

2026-08-28のcutover時に全旧galleryとの比較は`96/96`成功、最大差分`0.0614`でした。以後はreview済みのGo出力をregression oracleとして使用します。offlineではnetwork依存27件を除く`69/69`が成功します。case数とgroup別件数は`internal/gallery/cases_test.go`で固定し、CIの`go-gallery` jobが全96件を毎回比較します。

## Legacy cache oracle

fixtureはcutover前に公開されていた次のcache配置を再現します。

- `.makeitaquote/fonts/<slug>-vNN-<weight>-<id>.ttf`
- `.makeitaquote/twemoji/<codepoint>.png`
- `{version,count,installedAt}`形式のTwemoji `manifest.json`

Go版はこれらをlist/render/uninstallでき、同versionのTwemojiを再downloadしません。新しいschema 1 manifestと旧形式は並行して読めます。

## CI gate

GitHub ActionsはLinux、Windows、macOSで`go test ./...`と`go vet ./...`を実行します。Linuxでは追加でrace test、`CGO_ENABLED=0`の`nodynamic` test/build、全96件gallery比較を実行します。release後のgalleryと静的HTMLも`cmd/miq-gallery`で生成します。
