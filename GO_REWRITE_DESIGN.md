# MiQ Go実装設計書

> 状態: 2026-08-28 cutover完了。実装、CLI、全96件gallery、静的docs、CI、GitHub ReleaseをGoへ移行し、旧runtime/build/test資産とpackage metadataを削除した。`docs/visual`はreview済みGo出力をcanonical goldenとして継続利用する。

作成日: 2026-08-27
対象: `makeitaquote@10.3.2` (`main`)
移行先モジュール: `github.com/tikipiya/MiQ`
ルートパッケージ名: `miq`

## 1. 目的

MiQを、実行時・ビルド・テスト・補助コマンドまでGoだけで構成する。Node.js、npm、`@napi-rs/canvas`、JavaScriptランタイムを必要とせず、単一のGoライブラリと`miq`バイナリとして配布する。

この移行は機械的な翻訳ではない。現行の公開機能と出力特性を互換性契約として固定し、Goの型、`context.Context`、`io.Reader`/`io.Writer`、`image.Image`に合わせてAPIを再設計する。

### 1.1 完了条件

- 旧ソース、旧ビルド設定、旧package metadataをrepositoryに残さない。
- ローカル引用画像、会話画像、Discord/Misskey/X入力、Markdown/MFM処理、テーマ、フォント、絵文字、外部APIクライアント、CLIをGoだけで提供する。
- PNG/JPEG/WebP/AVIFを出力できる。
- Linux glibc/musl、Windows、macOSのamd64/arm64で`CGO_ENABLED=0`ビルドを基本経路にする。
- 現行の35テストファイル・688テストケースとビジュアルギャラリーを移植時の回帰基準として利用する。
- 最終CIにNode.jsのセットアップを残さない。

### 1.2 非目標

- npm利用者のTypeScriptソース互換、CommonJS/ESM互換をGoで再現すること。
- PNG等の圧縮バイト列を現行Skia出力と完全一致させること。
- ブラウザ上で直接レンダリングすること。
- GIF/WebP/AVIFのアニメーションを保持すること。入力アニメーションは従来どおり先頭フレームを使う。
- Voids APIそのものをサーバー側で再実装すること。クライアントのみ移植する。

## 2. 現行システムの基準点

調査時点の本番コードは64ファイル・約7,623行、テストは35ファイル・約5,205行である。公開面は次の6領域に分かれる。

| 領域 | 現行機能 | Go移行後 |
| --- | --- | --- |
| 単一引用 | `MiQ`、テーマ、各種setter、画像出力 | `Engine.RenderQuote` |
| 会話 | `MiQConversation`、連続発言のグループ化 | `Engine.RenderConversation` |
| 入力変換 | Discord、Misskey、X、各Markdown | `adapter/*`と`markup/*` |
| アセット | Google Fonts、Twemoji、avatar/emoji LRU | `asset`, `font`, `emoji` |
| 外部API | `/fakequote`、`/fakequotebeta` | `api/voids.Client` |
| CLI | install/uninstall/ls/search/outdated/update/prune/env/generate | `cmd/miq` |

維持する主な制約は以下である。

- 本文最大4,000文字、名前最大128文字、ウォーターマーク最大64文字。
- プリセットは`dark`、`light`、`color`、`portrait`、`portrait-light`、`custom`。
- 出力は`png`、`jpeg`/`jpg`、`webp`、`avif`。既定品質は92。
- `scale`は0より大きく8以下。
- Discordのメンション解決、Discord Markdown、MFM、CommonMarkの振る舞いを維持する。
- Twemoji、Discordカスタム絵文字、Misskeyカスタム絵文字を同一行に混在できる。
- キャッシュは正キャッシュ、負キャッシュ、TTL、LRU上限、in-flight合流を持つ。
- フォントとTwemojiの既定保存先はプロジェクト直下`.makeitaquote`、環境変数は`MIQ_FONT_CACHE_DIR`と`MIQ_TWEMOJI_CACHE_DIR`。
- Voidsの`/fakequote`はURLを返し、`/fakequotebeta`は画像を直接返す。この意味の違いを維持する。

## 3. 互換性方針

互換性を3段階に分ける。

### 3.1 必須: 意味互換

- 同じ入力から同じ本文、表示名、ユーザー名、avatar、絵文字URLを解決する。
- 同じテーマ値から同じ領域、比率、配置、折り返し、overflow方針を選ぶ。
- CLIのコマンド名、別名、フラグ、既定値、終了コード、JSON出力のキーを維持する。
- エラー原因をvalidation、asset、font、render、APIに分類できる。
- オフライン指定時にネットワークアクセスを行わない。

### 3.2 必須: 視覚互換

- 1倍出力でレイアウト境界を原則±1px以内にする。
- 改行位置、省略記号、絵文字位置、avatarのcrop/contain、グラデーション方向を一致させる。
- SkiaとGo rasterizerの差によるアンチエイリアス、hinting、JPEG/WebP/AVIF圧縮差は許容する。
- ギャラリー画像ごとにSSIM 0.98以上を初期合格値とし、文字部分は別途マスク比較と行構造比較を行う。単一指標だけで合否を決めない。

### 3.3 対象外: ソース互換

旧chain API、`Buffer`、Node `Readable`、CanvasオブジェクトはGoへ持ち込まない。公開面はGoの型、`io.Reader`、`image.Image`、`context.Context`に合わせる。

## 4. 技術方針

### 4.1 Goバージョンとビルド

- 最低Goバージョンは1.24とする。Phase 1の初期実装で、描画依存を互換commitに固定したうえで`go.mod`、unit test、`CGO_ENABLED=0` buildが成立することを確認する。
- 通常ビルドは`CGO_ENABLED=0`で通す。
- `go build ./...`、`go test ./...`、`go vet ./...`、`go test -race ./...`を標準検証とする。
- 外部コマンド、共有ライブラリ、WASMランタイムを必須条件にしない。

### 4.2 描画と文字組み

第一候補は`github.com/tdewolff/canvas`のpure-Go経路とする。複雑な文字のshaping、font fallback、CJK、合成bold、パス描画、rasterizerを一つの境界内で利用できるためである。ただしアプリケーション層から直接依存させず、`internal/draw`の小さなinterfaceで隔離する。初期実装ではGo 1.24互換の`v0.0.0-20250728095813-50d4cb1eee71`を固定した。最新版へ更新する際は最低Goバージョン変更を別途判断する。

文字組みの中核はpure GoのHarfBuzz実装を持つ`go-text/typesetting`系を使う。これは公式READMEでpure Goの高品質text shapingを目的とし、複数のGo UI toolkitで利用されている一方、v0系でAPI変化が速いと明記されている。そのため、依存型を公開APIに出してはならない。

参考:

- [tdewolff/canvas](https://github.com/tdewolff/canvas)
- [go-text/typesetting](https://github.com/go-text/typesetting)

実装前に次のspikeを必須にする。

1. Noto Sans JPで日本語、英語、結合文字、ZWJを描画する。
2. `dark`、`portrait`、円形avatar、alpha gradientを再現する。
3. 35種類以上のfont familyを繰り返し登録し、並行render時のraceとメモリを計測する。
4. Windows/Linux/macOSで同じ行分割になることを確認する。

spikeが不合格なら`internal/draw`の実装だけを`go-text/typesetting` + `golang.org/x/image/vector`の直接実装へ交換する。ドメイン、layout、asset、CLIは影響を受けない。

### 4.3 画像codec

| 形式 | 方針 |
| --- | --- |
| PNG | 標準`image/png` |
| JPEG/JPG | 標準`image/jpeg` |
| GIF入力 | 標準`image/gif`の先頭フレーム |
| WebP | `github.com/gen2brain/webp@v0.6.4`。`nodynamic` build tagで同梱pure-Go経路を固定可能 |
| AVIF | Go 1.24互換の`github.com/gen2brain/avif@v0.4.4`。配布サイズ・初期化時間を継続計測 |

Goの`golang.org/x/image/webp`はdecoderのみなので、出力互換のため別encoderが必要である。`gen2brain/webp`はlibwebpをpure Goへ変換したCGo-free fallbackを公式READMEで提供している。AVIFもcodecを`internal/codec`のinterfaceに閉じ込め、性能や保守性の問題が出たとき交換可能にする。

参考:

- [gen2brain/webp](https://github.com/gen2brain/webp)
- [golang.org/x/image/webp](https://pkg.go.dev/golang.org/x/image/webp)
- [gen2brain/avif](https://github.com/gen2brain/avif)

### 4.4 Unicode、Markdown、MFM

- grapheme clusterは`github.com/rivo/uniseg`を候補にし、Unicode versionを生成物に記録する。
- CommonMark/GFMは`github.com/yuin/goldmark`のASTをplain textへ変換する。
- Discord Markdownは現行テストを仕様として専用tokenizerをGoへ移植する。CommonMark parserを流用しない。
- MFMは必要nodeだけを扱う小型lexer/parserを実装する。未知nodeは中身を保持するsafe fallbackにする。
- BudouXのphrase boundaryは、互換ライセンスを確認したうえでモデルと最小推論器をGoに移植する。モデルは`go:embed`し、実行時にJavaScriptやPythonを呼ばない。
- 禁則処理、強制分割、ellipsis、改行候補の優先順位は現行`breakpoint.ts`/`wrap.ts`のテストをそのままtable test化する。

### 4.5 HTTP

- 標準`net/http`だけを利用する。
- 全I/O APIの第1引数は`context.Context`とする。
- 共通clientにconnect/header/body timeout、最大response bytes、redirect上限、retry、User-Agentを設定する。
- retry対象は接続失敗、408、429、5xxの冪等GETのみ。Voids POSTは既定でretryせず、明示設定時だけ行う。
- `http.Client`とtransportを注入可能にし、テストは`httptest.Server`で完結させる。

## 5. 全体アーキテクチャ

```text
Caller / cmd/miq
        |
        v
  Public Go API (miq)
        |
        +--> adapters -------- Discord / Misskey / X / Markdown
        |
        v
  validate -> normalize -> segment -> asset prefetch
        |                       |
        |                       +--> font / emoji / avatar stores
        v
  measure -> line break -> fit -> layout
        |
        v
  draw backend -> image.NRGBA -> codec -> io.Writer / []byte

  api/voids.Client は上記ローカル描画経路と独立
```

依存方向は`cmd -> public package -> internal use cases -> domain`とし、`internal`から`cmd`や外部API packageを参照しない。ネットワーク、時計、filesystem、codec、drawing backendはinterface越しに注入する。

## 6. ディレクトリ構成

```text
makeitaquote/
├── go.mod
├── go.sum
├── README.md
├── MIGRATING_GO.md
├── THIRD-PARTY-NOTICES.md
├── cmd/
│   └── miq/
│       └── main.go
├── api/
│   └── voids/
│       ├── client.go
│       ├── types.go
│       └── client_test.go
├── adapter/
│   ├── discord/
│   ├── misskey/
│   └── twitter/
├── markup/
│   ├── commonmark/
│   ├── discord/
│   └── mfm/
├── asset/
│   ├── fonts.go
│   ├── twemoji.go
│   └── info.go
├── theme/
│   ├── types.go
│   ├── presets.go
│   └── color.go
├── miq.go
├── types.go
├── errors.go
├── encode.go
├── internal/
│   ├── app/             # render use cases
│   ├── cache/           # generic TTL/LRU + in-flight合流
│   ├── codec/           # PNG/JPEG/WebP/AVIF adapter
│   ├── draw/            # canvas/rasterizer adapter
│   ├── emoji/           # Unicode/Discord/Misskey segment
│   ├── font/            # load/register/fallback/download
│   ├── imageio/         # source解決、decode、resize/crop/filter
│   ├── layout/          # quote/conversation geometry
│   ├── linebreak/       # phrase/grapheme/kinsoku
│   ├── text/            # measure/wrap/fit/draw model
│   └── testutil/
├── assets/
│   ├── catalogue.json
│   └── unicode/         # 生成済みemoji/line-break tables
├── testdata/
│   ├── fixtures/
│   ├── golden/
│   └── compatibility/
├── tools/
│   ├── visualcheck/
│   └── gallery/
├── docs/                # Go toolが生成する静的HTML/画像
└── .github/workflows/
    ├── ci.yml
    └── release.yml
```

`internal`の型を公開APIから返さない。公開subpackageは利用者が直接使う価値のある`theme`、`asset`、`adapter`、`markup`、`api/voids`だけに限定する。

## 7. 公開Go API

### 7.1 Engine

```go
package miq

type Engine struct { /* private */ }

type EngineOptions struct {
    HTTPClient      *http.Client
    FontCacheDir    string
    TwemojiCacheDir string
    Offline         bool
    StrictFonts     bool
    MissingEmoji    MissingEmojiBehavior
    NetworkPolicy   NetworkPolicy
    Logger          *slog.Logger
}

func NewEngine(opts EngineOptions) (*Engine, error)

func (e *Engine) RenderQuote(
    ctx context.Context,
    quote Quote,
    opts RenderOptions,
) (*image.NRGBA, error)

func (e *Engine) RenderConversation(
    ctx context.Context,
    messages []ConversationMessage,
    opts ConversationOptions,
) (*image.NRGBA, error)

func (e *Engine) WriteQuote(
    ctx context.Context,
    w io.Writer,
    quote Quote,
    opts RenderOptions,
    enc EncodeOptions,
) error
```

`Engine`は初期化後concurrent-safeとする。font registry、LRU、singleflight相当の状態を共有する。`Quote`とoption値はcall単位でcopyし、render中に利用者データを変更しない。

### 7.2 Domain型

```go
type Quote struct {
    Text        string
    Avatar      ImageSource
    Username    string
    DisplayName string
    Watermark   string
}

type ImageSource interface {
    isImageSource()
}

func ImageURL(u *url.URL) ImageSource
func ImageFile(path string) ImageSource
func ImageBytes(b []byte) ImageSource
func ImageValue(img image.Image) ImageSource

type RenderOptions struct {
    Theme    theme.Input
    Scale    float64
    Misskey  MisskeyOptions
    AutoFont AutoFontOptions
}

type EncodeOptions struct {
    Format  Format
    Quality int
}
```

空の`ImageSource`はavatarなしを表す。URL、file、bytesを曖昧なstring一つで兼用しない。`[]byte`はconstructorでcopyして、render中の変更競合を防ぐ。

### 7.3 使用例

```go
engine, err := miq.NewEngine(miq.EngineOptions{})
if err != nil {
    return err
}

avatarURL, err := url.Parse("https://example.com/avatar.png")
if err != nil {
    return err
}

err = engine.WriteQuote(ctx, out, miq.Quote{
    Text:        "小さな工夫が、毎日の使いやすさをつくる。",
    Avatar:      miq.ImageURL(avatarURL),
    Username:    "sample_user",
    DisplayName: "Sample User",
}, miq.RenderOptions{
    Theme: theme.Preset("dark"),
}, miq.EncodeOptions{
    Format: miq.PNG,
})
```

### 7.4 adapter

各SNS SDKへの依存を避けるため、adapterは最小構造体を受ける。GoではTypeScriptのduck typingがないため、外部SDK専用adapterは別moduleにせず利用者側で変換しやすいJSON互換型を提供する。

```go
q, err := discord.QuoteFromMessage(message, discord.Options{
    Avatar:          discord.GuildAvatar,
    Name:            discord.Nickname,
    ResolveMentions: true,
    Locale:          "ja-JP",
    TimeZone:        "Asia/Tokyo",
})
```

X公式APIとFxTwitterの変換は`adapter/twitter`に残す。Misskeyはlocal/remote handle、CW優先、MFM除去、custom emoji mapを維持する。

## 8. Theme設計

- `theme.Theme`は完全解決済みの値、`theme.Input`はpointer fieldを使う部分上書き値とする。
- `Input.Extends`でpresetを継承する。
- 0を有効値にできる項目があるため、部分指定にzero-valueだけを使わない。
- colorは`color.NRGBA`に正規化し、hex、CSS name、rgb(a)、hsl(a)、整数入力用のparse helperを公開する。
- 0より大きく1以下のsizeはcanvas比、それより大きい値はpixelという現行規則を維持する。
- theme resolve後にvalidationし、field path付き`ValidationError`を返す。
- preset値はGo sourceに固定し、変更をsnapshot testで検出する。

背景描画順は固定する。

1. solid background
2. generated gradient
3. background image + opacity
4. avatar
5. avatar fade gradient
6. quote marks/text/divider/attribution/watermark

## 9. Render pipeline

### 9.1 単一引用

1. inputとthemeをcopyする。
2. normalize/validateする。
3. 本文をplain text/emoji segment列へ変換する。
4. 必要文字のfont fallbackを決める。
5. avatar、emoji、fontを上限付きで並列prefetchする。
6. font sizeごとにshape/measureし、binary searchでfitする。
7. line break、overflow、ellipsisを確定する。
8. layout座標を計算する。
9. `image.NRGBA`へ決められた順序で描画する。
10. callerへimageを返すか、指定codecで`io.Writer`へencodeする。

各段階は副作用のない値変換を基本にし、I/Oはprefetchとencodeに集約する。

### 9.2 会話

- messageは入力順を維持する。
- 連続する同一`Username`を1 groupにまとめる。
- group先頭だけavatarと名前を表示する。
- 幅はoption、heightは内容から算出する。
- 最大画像高さと最大message数をvalidationで制限し、巨大割り当てを防ぐ。
- dark/lightの2presetはquote themeから独立させる。

### 9.3 Text fit

- 改行候補の優先順位は明示改行、空白、phrase boundary、一般grapheme境界、強制分割の順。
- 行頭禁則、行末禁則を適用する。
- URL等のoversized tokenはgrapheme単位で必ず分割可能にする。
- `overflow=shrink`はmin sizeまで二分探索する。
- `overflow=ellipsis`は最終行末からsegmentを削り、ellipsis自体を再計測する。
- emojiはbaselineに沿うinline replaced elementとしてmeasureとdrawで同じadvanceを使う。
- measure結果はfont identity、weight、size、textでrequest-local cacheする。

## 10. Font

### 10.1 Registry

- Engineごとのregistryとし、package globalな変更可能状態を避ける。
- family、weight、italic、source hashをkeyにする。
- font bytesはfont objectが参照する期間保持する。
- system font scan結果は一度だけ作り、read-only snapshotにする。
- real boldがなければ現行と同じsynthetic boldを適用する。

### 10.2 Auto download

- Google Fonts CSS/metadataを標準HTTP clientで取得する。
- TTFを優先し、取得したfileはSHA-256とsizeを検証する。
- 同一directoryに一時fileを書いて`fsync`後atomic renameする。
- cache filenameにfamily/version/weight/style/hashを含める。
- `strictFonts=false`ではwarningを記録してfallback、`true`では`FontError`。
- font license metadataを保存し、`THIRD-PARTY-NOTICES.md`生成に利用する。

## 11. Emojiと画像asset

### 11.1 Segment

- Unicode emojiはgrapheme clusterと生成済みUnicode/Twemoji tableで検出する。
- variation selector、skin tone、keycap、flag、ZWJ sequenceを一つのsegmentにする。
- Discord custom emojiは`<(a)?:name:id>`を解析する。
- Misskey custom emojiはhost/nameを解決し、provided mapを最優先する。
- source textをsegmentに保持し、取得失敗時に`ignore`、`text`、`error`を選べるようにする。

### 11.2 Cache

avatarとemojiに独立したcacheを持たせる。

```go
type CacheOptions struct {
    Enabled     bool
    MaxEntries  int
    TTL         time.Duration
    NegativeTTL time.Duration
}
```

- value cacheとfailure cacheは別LRU。
- 同じkeyの同時loadはsingleflightで1回に合流する。
- cache keyは正規化URL、decode option、requested sizeを含む。
- entry数だけでなくdecode後byte数にも上限を設ける。
- clear/info/configureをEngine methodとして提供する。

### 11.3 Network安全性

- `http`/`https`以外のremote schemeを拒否する。
- response bodyの上限、画像dimension、総pixel数をdecode前後に検査する。
- redirect先にも同じpolicyを適用する。
- loopback、link-local、private networkは既定で拒否し、self-hosted Misskey等は`AllowPrivateNetwork`で明示許可する。
- local fileはURLとして解釈せず、`ImageFile`で明示されたものだけ読む。
- errorやlogにauthorization header、query token、body全体を出さない。

## 12. Encodeとstream

- 公開image型は`*image.NRGBA`に統一する。
- `Encode(w, img, options)`と`EncodeBytes(img, options)`を提供する。
- Goの`io.Writer`がNodeの`toStream()`を置き換える。全画像を追加copyしてからstream風に返さない。
- data URLは`EncodeDataURL` helperとして残すが、大容量用途には非推奨と記載する。
- PNGはqualityを無視する。JPEG/WebP/AVIFは1–100をvalidationする。
- alphaを持つ画像のJPEG化ではtheme backgroundへ合成し、黒への暗黙変換を避ける。

## 13. Voids API client

`api/voids`はlocal rendererから独立し、画像描画依存をimportしない。

```go
type Client struct { /* private */ }

func NewClient(opts Options) (*Client, error)
func (c *Client) HostedURL(ctx context.Context, q Quote) (*url.URL, error)
func (c *Client) Direct(ctx context.Context, q Quote) ([]byte, error)
func (c *Client) HostedBytes(ctx context.Context, q Quote) ([]byte, error)
```

- `HostedURL`: POST `/fakequote`、`{url}`を返す。
- `Direct`: POST `/fakequotebeta`、binaryを1往復で返す。
- `HostedBytes`: hosted URL取得後GETする2往復の明示API。
- wireは`display_name`等のsnake_caseを維持する。
- status、endpoint、制限長のresponse bodyを持つ`APIError`を`errors.As`できる。
- base URL、timeout、header、clientを注入可能にする。

## 14. CLI

標準`flag`だけではsubcommand aliasとhelp生成が不足するため、`github.com/spf13/cobra`を候補とする。ただしcommand本体はlibrary関数へ分離し、parser依存をテストの中心にしない。

維持するcommand:

| command | aliases | 概要 |
| --- | --- | --- |
| `install` | `add`, `i` | Twemoji/font取得 |
| `uninstall` | `remove`, `rm`, `r`, `un`, `unlink` | asset削除 |
| `ls` | `list` | install状況。`--json`対応 |
| `search` | `find`, `s` | font検索 |
| `outdated` | - | version確認 |
| `update` | - | Twemoji/font更新 |
| `prune` | - | 古いfont削除 |
| `env` | `doctor` | storage/network診断 |
| `generate` | `render` | 引用画像生成 |

`generate`の既存flagと既定値を維持する。stdoutは成果/JSON、stderrはwarning/progress/errorに分離する。終了コードは0成功、1操作失敗、2usage errorとする。`--json`時はstdoutへJSON以外を混ぜない。

配布経路:

- `go install github.com/tikipiya/MiQ/cmd/miq@latest`
- GitHub ReleasesにOS/arch別archiveとSHA-256 checksum
- Linux、Windows、macOSの署名/attestationをrelease workflowで生成

## 15. Error設計

```go
var (
    ErrValidation = errors.New("validation error")
    ErrAsset      = errors.New("asset error")
    ErrFont       = errors.New("font error")
    ErrRender     = errors.New("render error")
    ErrAPI        = errors.New("api error")
)

type FieldError struct {
    Field string
    Err   error
}
```

- 全errorを`fmt.Errorf("...: %w", err)`でwrap可能にする。
- callerは`errors.Is`で分類、`errors.As`でfield/status/URL等の詳細を得る。
- cancellationとdeadlineは`context.Canceled`/`context.DeadlineExceeded`を失わない。
- 複数asset取得失敗は必須assetだけをerrorにし、fallback可能な失敗はstructured warningとして返す。
- panicはprogrammer bugか標準libraryの不変条件違反に限定し、利用者入力では返さない。

## 16. 並行性とresource管理

- `Engine`はconcurrent-safe、render callは独立。
- 1 renderあたりのdownload並列数は既定8、Twemoji一括installは既定16。
- global semaphoreでprocess全体の外部I/Oと重いcodec処理を制限する。
- `context`取消時は新規download/shape/encodeを止める。
- goroutineをfire-and-forgetしない。全goroutineは`errgroup`相当でjoinする。
- 最大canvas width/height、pixel数、入力byte数、message数、emoji数を定数化し、validation前に大容量確保しない。
- benchmarkでrender/sec、p50/p95、allocs、peak RSS、cold font loadを記録する。

## 17. 永続化とoffline

- cache root探索は最寄りの`go.mod`を基準とし、見つからない場合はcurrent directoryを使う。旧`.makeitaquote` directoryはそのまま再利用する。
- font/Twemoji manifestにschema version、upstream version、installed time、file hashを持たせる。
- 旧manifestをreadできるmigration readerを1 major version維持する。
- `Offline=true`ではDNSを含む全network code pathへ入らないことをfake transportで検証する。
- installはresume可能、既存のnon-emptyかつhash一致fileをskipする。
- uninstallはCLIでtargetをresolve/表示してから、`.makeitaquote/fonts`または`twemoji`だけを削除する。cache root全体を曖昧に削除しない。

## 18. Test戦略

### 18.1 Unit test

現行688 testを一対一の表にし、各testを次のpackageへ移す。

- validation、source adapter、mention、tweet/note
- color/theme resolve/layout
- grapheme/segment/breakpoint/wrap/fit
- font catalogue/cache/install/update
- emoji/Twemoji/cache
- avatar crop/filter
- codec、API、CLI

日時依存はclock、network依存はRoundTripper、filesystem依存は`t.TempDir()`を注入する。test内で外部networkを使わない。

### 18.2 Golden/visual test

- review済みのGo出力`docs/visual`をcanonical referenceとして固定する。
- 決定的fixtureをGoへ読み込ませ、寸法、manifest、pixel imageを保存する。
- pixel diff、SSIM、perceptual hash、alpha maskを組み合わせる。
- 意図的な差はreview済みallowlistへ理由と期限を記録する。
- Linuxの固定font環境をcanonical golden generatorとし、他OSは構造検証を主にする。

### 18.3 Fuzz test

- color parser
- Discord/MFM/CommonMark stripper
- mention/emoji tokenizer
- image header/decode boundary
- theme JSON decoder
- line breaker

不変条件はpanicしない、無限loopしない、UTF-8を壊さない、出力dimensionが上限内、同一入力で決定的であること。

### 18.4 Race/compatibility

- `go test -race ./...`で同一Engineへの並行render、cache clear/info、font登録を実行する。
- Windows、macOS、Ubuntu glibc、Alpine相当musl containerでCIする。
- `CGO_ENABLED=0`を明示し、誤ってCGo依存が入った場合CIを落とす。

## 19. CI/CD

### CI job

1. `gofmt -l`が空であること。
2. `go vet ./...`。
3. `go test ./...`をOS matrixで実行。
4. Linuxで`go test -race ./...`。
5. `CGO_ENABLED=0 go build ./...`をOS/arch matrixで実行。
6. canonical environmentでgolden test。
7. `govulncheck ./...`、license allowlist、`go mod verify`。
8. CLI smoke testとrelease archive展開後の`miq --version`。

### Release

- SemVer tagからGoReleaser相当のGo製release toolまたはworkflowを使う。
- source archive、binary archive、checksum、SBOM、provenanceを公開する。
- gallery generatorもGo commandとし、生成後`docs`差分だけcommitする。
- 手書き`docs/app.js`は廃止し、Go toolが生成するJavaScript不要の静的galleryへ置換する。

## 20. Cutover結果

移行工程は完了した。現在のrepositoryは次の状態を正とする。

- module pathは`github.com/tikipiya/MiQ`。
- quote、conversation、adapter、markup、asset、font、emoji、Voids API client、CLIをGoで提供する。
- gallery generatorは96件を生成し、`docs/visual`との全件比較を行う。
- docsはGo toolが生成するJavaScript不要の静的HTMLである。
- CIはtest、vet、race、golden、`CGO_ENABLED=0` buildを実行する。
- release workflowは対象platformのbinary archiveとchecksumを生成する。
- 旧runtime、旧package metadata、旧画像素材は削除済みである。

完了判定は、clean checkoutでGo toolchainだけを使い、build、test、gallery生成、release dry-runが成功することとする。

## 21. Acceptance criteria

リリース候補は以下をすべて満たす必要がある。

- 全移植unit test、fuzz smoke、race test、golden testが成功。
- `CGO_ENABLED=0`で対象OS/archをbuild可能。
- CLI command/alias/flag/JSON/exit code互換。
- offline testでnetwork callが0。
- PNG/JPEG/WebP/AVIFをdecodeし直せ、dimensionとalpha/quality契約が正しい。
- 代表的な日本語、英語、絵文字混在出力のline breakが現行と一致。
- cold/warm benchmarkが記録され、1倍標準画像のp95が合意したbudget内。
- binary、cache、font license、Twemoji licenseをSBOM/noticeで追跡可能。
- repositoryに必須の`.ts`/`.js`/`package.json`/`node_modules`依存が残っていない。

## 22. 主なriskと対策

| Risk | 影響 | 対策 |
| --- | --- | --- |
| SkiaとGo rasterizerの字形差 | visual diff増加 | 構造比較と知覚比較を分離、固定font、backend隔離 |
| text libraryがv0でAPI変化 | build破壊 | `internal/draw`に封じ、version pin、adapter test |
| AVIFの初期化/配布size | CLI肥大・cold start | codec分離、benchmark、必要ならbuild tag付き軽量版を追加。ただし標準配布は全形式対応 |
| BudouXのGo公式実装不在 | 改行差 | 最小推論器とmodelをlicense付きで内包、現行testをoracle化 |
| 旧package利用者の移行断絶 | 利用者離脱 | Go examples、公開API文書、明確なversioning |
| system font差 | OSごとの画像差 | 自動font/cacheを優先、canonical goldenは固定font |
| remote assetによるSSRF/巨大画像 | security/DoS | network policy、byte/pixel/redirect上限、context |
| global font/cache race | crash/不定出力 | Engine ownership、immutable snapshot、race CI |

## 23. Cutover decision record

1. 同一repositoryを`github.com/tikipiya/MiQ` Go moduleへ切り替え、旧package実装はrepositoryから削除した。
2. 最低versionはGo 1.24とした。
3. 描画backendは`internal/draw`境界内の`tdewolff/canvas`を採用した。
4. WebP/AVIFは`nodynamic` tagを含むCGOなしbuildをrelease標準とした。
5. remote画像のprivate network取得は既定拒否とした。
6. visual regressionはdimension完全一致と30×16 block RGB平均差`0.08`以下を契約とした。
7. module pathにmajor suffixがないため、現行release workflowはGo moduleの`v0`/`v1` tagだけを受け付ける。
