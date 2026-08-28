package gallery

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"net/url"
	"strings"
	"time"

	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/adapter/discord"
	"github.com/tikipiya/MiQ/adapter/misskey"
	"github.com/tikipiya/MiQ/adapter/twitter"
	"github.com/tikipiya/MiQ/asset"
	"github.com/tikipiya/MiQ/markup/commonmark"
	"github.com/tikipiya/MiQ/theme"
)

const (
	shortText = "余白が言葉を引き立てる"
	jaText    = "小さな工夫が、毎日の使いやすさをつくる。"
	longText  = "伝えたい内容を丁寧に整えると、短い文章でも読みやすくなります。文字の大きさや行間、余白の取り方を少し変えるだけで、同じ言葉から受ける印象も自然に変わります。"
	english   = "Clear typography gives every sentence enough room to be read, understood, and remembered."
)

func pointer[T any](value T) *T { return &value }

type quoteBuilder func(*Runtime) (miq.Quote, miq.RenderOptions, error)

func quoteCase(group, name string, build quoteBuilder, modify ...func(*Case)) Case {
	c := Case{Group: group, Name: name, Format: miq.PNG, Render: func(ctx context.Context, runtime *Runtime) (image.Image, error) {
		quote, options, err := build(runtime)
		if err != nil {
			return nil, err
		}
		return runtime.Engine.RenderQuote(ctx, quote, options)
	}}
	for _, fn := range modify {
		fn(&c)
	}
	return c
}

func conversationCase(group, name string, messages func(*Runtime) ([]miq.ConversationMessage, miq.ConversationOptions, error), modify ...func(*Case)) Case {
	c := Case{Group: group, Name: name, Format: miq.PNG, Render: func(ctx context.Context, runtime *Runtime) (image.Image, error) {
		items, options, err := messages(runtime)
		if err != nil {
			return nil, err
		}
		return runtime.Engine.RenderConversation(ctx, items, options)
	}}
	for _, fn := range modify {
		fn(&c)
	}
	return c
}

func attributed(runtime *Runtime, text string, avatar miq.ImageSource) miq.Quote {
	return miq.Quote{Text: text, Avatar: avatar, Username: "sample_user", DisplayName: "Sample User", Watermark: "Make it a Quote"}
}

func local(runtime *Runtime, text string) (miq.Quote, miq.RenderOptions, error) {
	return attributed(runtime, text, miq.ImageFile(runtime.Assets.PNG)), miq.RenderOptions{}, nil
}

func configured(text string, avatar func(*Runtime) miq.ImageSource, options miq.RenderOptions) quoteBuilder {
	return func(runtime *Runtime) (miq.Quote, miq.RenderOptions, error) {
		return attributed(runtime, text, avatar(runtime)), options, nil
	}
}

func png(runtime *Runtime) miq.ImageSource   { return miq.ImageFile(runtime.Assets.PNG) }
func photo(runtime *Runtime) miq.ImageSource { return miq.ImageFile(runtime.Assets.JPG) }
func noAvatar(*Runtime) miq.ImageSource      { return nil }

func remote(runtime *Runtime) miq.ImageSource {
	parsed, _ := url.Parse(runtime.Assets.RemoteURL)
	return miq.ImageURL(parsed)
}

func note(value string) func(*Case) { return func(c *Case) { c.Note = value } }
func network(c *Case)               { c.Network = true }
func format(value miq.Format, quality int) func(*Case) {
	return func(c *Case) { c.Format, c.Quality = value, quality }
}
func dimensions(width, height int) func(*Case) {
	return func(c *Case) { c.ExpectWidth, c.ExpectHeight = width, height }
}

// Cases is the canonical 96-case gallery registry. Its order is part of the
// published manifest contract.
func Cases() []Case {
	var cases []Case
	add := func(items ...Case) { cases = append(cases, items...) }

	// 01-themes
	for _, item := range []struct {
		name   string
		preset theme.Name
		text   string
	}{
		{"dark (default)", theme.Dark, longText}, {"light", theme.Light, longText},
		{"color (avatar keeps its color)", theme.Color, longText}, {"portrait", theme.Portrait, shortText},
		{"portrait-light", theme.PortraitLight, shortText},
	} {
		item := item
		add(quoteCase("01-themes", item.name, configured(item.text, png, miq.RenderOptions{Theme: theme.Preset(item.preset)})))
	}

	// 02-layout
	add(
		quoteCase("02-layout", "avatar left (default)", configured(longText, png, miq.RenderOptions{})),
		quoteCase("02-layout", "avatar right — text, gradient and watermark all follow", configured(longText, png, miq.RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{Position: pointer(theme.Right)}}}), note("text.area and watermark.position are 'auto' by default")),
		quoteCase("02-layout", "narrow avatar", configured(longText, png, miq.RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{WidthRatio: pointer(.32)}, Gradient: &theme.GradientInput{StartRatio: pointer(.14), EndRatio: pointer(.32)}}})),
		quoteCase("02-layout", "stacked on a landscape canvas", configured(shortText, photo, miq.RenderOptions{Theme: theme.Input{Extends: theme.Portrait, Width: pointer(1280), Height: pointer(720)}})),
		quoteCase("02-layout", "no gradient", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{Gradient: &theme.GradientInput{Enabled: pointer(false)}}})),
		quoteCase("02-layout", "circular avatar", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{Shape: pointer(theme.Circle), WidthRatio: pointer(.32)}, Gradient: &theme.GradientInput{Enabled: pointer(false)}}}), note("clipped to the largest circle that fits the box; the fallback tile matches")),
	)

	// 03-text
	kinsoku := "準備できましたか？「はい」と答えて、次へ進みます（ゆっくりで大丈夫です）。"
	veryLong := strings.Repeat(jaText, 40)
	add(
		quoteCase("03-text", "short", configured(shortText, png, miq.RenderOptions{})),
		quoteCase("03-text", "long — wraps and shrinks", configured(longText, png, miq.RenderOptions{})),
		quoteCase("03-text", "english wraps at spaces", configured(english, png, miq.RenderOptions{})),
		quoteCase("03-text", "kinsoku — no stranded punctuation", configured(kinsoku, png, miq.RenderOptions{})),
		quoteCase("03-text", "phraseBreak off — breaks per character", configured(longText, png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{PhraseBreak: pointer(false)}}}), note("compare with \"long — wraps and shrinks\"")),
		quoteCase("03-text", "explicit newlines", configured("考える\n整える\n届ける", png, miq.RenderOptions{})),
		quoteCase("03-text", "long url is force-broken", configured("使い方は https://github.com/tikipiya/MiQ/blob/main/README.md で確認できます", png, miq.RenderOptions{})),
		quoteCase("03-text", "overflow: ellipsis (default)", configured(veryLong, png, miq.RenderOptions{})),
		quoteCase("03-text", "overflow: shrink", configured(veryLong, png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{Overflow: pointer(theme.Shrink)}}})),
		quoteCase("03-text", "left aligned", configured(longText, png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{Align: pointer(theme.AlignLeft)}}})),
	)

	// 04-emoji
	add(
		quoteCase("04-emoji", "twemoji, including ZWJ and skin tone", configured("完成しました✅ みんなで🎉 確認して👍🏽 次へ進もう🚀", png, miq.RenderOptions{}), network),
		quoteCase("04-emoji", "discord custom emoji", func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			return attributed(r, "ステータス "+strings.Join(r.Assets.DiscordEmoji, " ")+" 更新済み", png(r)), miq.RenderOptions{}, nil
		}, network, note("real ids from assets/discordemoji.json")),
		quoteCase("04-emoji", "misskey custom emoji", func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			return attributed(r, "進捗 "+strings.Join(r.Assets.Misskey.Emoji, " ")+" 共有します", png(r)), miq.RenderOptions{Misskey: miq.MisskeyOptions{Instances: []string{r.Assets.Misskey.Instance}}}, nil
		}, network),
		quoteCase("04-emoji", "all three together", func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			return attributed(r, "✅ "+r.Assets.DiscordEmoji[0]+" "+r.Assets.Misskey.Emoji[0]+" すべて表示できます", png(r)), miq.RenderOptions{Misskey: miq.MisskeyOptions{Instances: []string{r.Assets.Misskey.Instance}}}, nil
		}, network),
		quoteCase("04-emoji", "misskey off — drawn as plain text", func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			return attributed(r, "進捗 "+strings.Join(r.Assets.Misskey.Emoji, " ")+" 共有します", png(r)), miq.RenderOptions{}, nil
		}, note("Misskey emoji only resolve when an instance is configured")),
		quoteCase("04-emoji", "unfetchable emoji falls back to its source text", configured("未取得の絵文字 <:missing:123456789012345678> は文字として残ります", png, miq.RenderOptions{}), network),
	)

	// 05-typography
	wrapping := "文章の区切りを意識して折り返すと、限られた幅でも内容を追いやすくなります。"
	add(
		quoteCase("05-typography", "normal (default)", configured(wrapping, png, miq.RenderOptions{})),
		quoteCase("05-typography", "bold", configured(wrapping, png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{Weight: pointer(700)}}}), note("emulated by stroking when the font has no bold face")),
		quoteCase("05-typography", "weight 900", configured(wrapping, png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{Weight: pointer(900)}}})),
		quoteCase("05-typography", "everything bold", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{Weight: pointer(700)}, DisplayName: &theme.LabelInput{Weight: pointer(700)}, Username: &theme.LabelInput{Weight: pointer(700)}, Watermark: &theme.LabelInput{Weight: pointer(700)}}})),
	)

	// 06-fonts
	for _, family := range asset.FontCatalogue {
		family := family
		add(quoteCase("06-fonts", family, configured(family+"\n読みやすい文字 Sample 123", png, miq.RenderOptions{Theme: theme.Input{Text: &theme.TextInput{Font: pointer(family + ", Noto Sans JP, sans-serif")}}}), network))
	}

	// 07-quotes
	add(
		quoteCase("07-quotes", "none (default)", configured(jaText, png, miq.RenderOptions{})),
		quoteCase("07-quotes", "inline", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{QuoteMark: &theme.QuoteMarkInput{Display: pointer(theme.QuoteInline)}}})),
		quoteCase("07-quotes", "inline with 「」", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{QuoteMark: &theme.QuoteMarkInput{Display: pointer(theme.QuoteInline), Open: pointer("「"), Close: pointer("」")}}})),
		quoteCase("07-quotes", "block", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{QuoteMark: &theme.QuoteMarkInput{Display: pointer(theme.QuoteBlock)}}})),
		quoteCase("07-quotes", "divider", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{Divider: &theme.DividerInput{Enabled: pointer(true)}}})),
	)

	// 08-avatar
	add(
		quoteCase("08-avatar", "illustration (png with alpha)", configured(jaText, png, miq.RenderOptions{})),
		quoteCase("08-avatar", "photo (jpg)", configured(jaText, photo, miq.RenderOptions{})),
		quoteCase("08-avatar", "color kept", configured(jaText, photo, miq.RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{Grayscale: pointer(false)}}})),
		quoteCase("08-avatar", "from a remote url", configured(jaText, remote, miq.RenderOptions{}), network),
		quoteCase("08-avatar", "from a Buffer", func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			return attributed(r, jaText, miq.ImageBytes(r.Assets.PNGBytes)), miq.RenderOptions{}, nil
		}),
		quoteCase("08-avatar", "none — fallback tile with initial", configured(jaText, noAvatar, miq.RenderOptions{})),
		// The JS builder catches an unreachable URL and renders the same fallback tile.
		quoteCase("08-avatar", "unreachable url — same fallback", configured(jaText, noAvatar, miq.RenderOptions{})),
		quoteCase("08-avatar", "no fallback tile at all", configured(jaText, noAvatar, miq.RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{HasFallback: pointer(false)}}})),
	)

	// 10-formats
	add(
		quoteCase("10-formats", "png", configured(jaText, png, miq.RenderOptions{}), format(miq.PNG, 0)),
		quoteCase("10-formats", "jpeg q40 (visibly lossy)", configured(jaText, png, miq.RenderOptions{}), format(miq.JPEG, 40)),
		quoteCase("10-formats", "webp q90", configured(jaText, png, miq.RenderOptions{}), format(miq.WebP, 90)),
		quoteCase("10-formats", "avif q60", configured(jaText, png, miq.RenderOptions{}), format(miq.AVIF, 60)),
	)

	add(discordCases()...)
	add(misskeyCases()...)
	add(twitterCases()...)

	// 15-markdown
	markdown := "# A **clear message**\n\nText can come from a document, an issue, or a social post.\n\n- plain CommonMark\n- [a link](https://example.com) becomes just its label"
	add(
		quoteCase("15-markdown", "plain text, quoted as written", configured(markdown, png, miq.RenderOptions{}), note(".setText() does not strip markdown on its own — compare with the next card")),
		quoteCase("15-markdown", "stripMarkdown(text) composed with setText", configured(commonmark.Strip(markdown), png, miq.RenderOptions{})),
	)

	add(colorCases()...)

	// 09-sizing
	add(
		quoteCase("09-sizing", "default (630 tall)", configured(jaText, png, miq.RenderOptions{}), dimensions(0, 630)),
		quoteCase("09-sizing", "scale 0.5", configured(jaText, png, miq.RenderOptions{Scale: .5}), dimensions(0, 315), note("the same layout at half the resolution")),
		quoteCase("09-sizing", "scale 2", configured(jaText, png, miq.RenderOptions{Scale: 2}), dimensions(0, 1260)),
		quoteCase("09-sizing", "sized to the avatar height", configured(jaText, png, miq.RenderOptions{SizeToAvatar: miq.AvatarNativeHeight}), note("the avatar is drawn at its native resolution, never resampled")),
		quoteCase("09-sizing", "avatar contained rather than cropped", configured(jaText, photo, miq.RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{Fit: pointer(theme.Contain)}}})),
	)

	add(conversationCases()...)
	return cases
}

func discordCases() []Case {
	markdown := "**装飾された文章**は入力どおりに扱います\n- 箇条書きも\n-# 小さい見出しも"
	mentions := discord.Mentions{Users: map[string]discord.User{"1": {Username: "sample_user"}}, Channels: map[string]discord.Named{"2": {Name: "雑談"}}, Roles: map[string]discord.Named{"3": {Name: "メンバー"}}}
	from := func(message func(*Runtime) discord.Message, options discord.Options) quoteBuilder {
		return func(runtime *Runtime) (miq.Quote, miq.RenderOptions, error) {
			q, err := discord.FromMessage(message(runtime), options)
			return q, miq.RenderOptions{}, err
		}
	}
	minimal := func(content string) func(*Runtime) discord.Message {
		return func(*Runtime) discord.Message {
			return discord.Message{Content: content, Author: discord.User{Username: "someone"}}
		}
	}
	mentionMessage := func(*Runtime) discord.Message {
		return discord.Message{Content: "確認をお願いします <@1>。今日は <#2> で <@&3> と作業します", Author: discord.User{Username: "someone"}, Mentions: mentions}
	}
	return []Case{
		quoteCase("11-discord", "from a discord.js v14 message", from(func(r *Runtime) discord.Message {
			return discord.Message{Content: "更新しました " + r.Assets.DiscordEmoji[0], Author: discord.User{Username: "sample_user", GlobalName: "Sample User", Discriminator: "0", AvatarURL: r.Assets.RemoteURL}, Member: &discord.Member{DisplayName: "Project Member", AvatarURL: r.Assets.RemoteURL}}
		}, discord.Options{}), network),
		quoteCase("11-discord", "from a legacy message (discriminator kept)", from(func(r *Runtime) discord.Message {
			return discord.Message{Content: jaText, Author: discord.User{Username: "sample_user", Discriminator: "6666", AvatarURL: r.Assets.RemoteURL}}
		}, discord.Options{}), network),
		quoteCase("11-discord", "from a minimal message (no avatar)", from(minimal(jaText), discord.Options{})),
		quoteCase("11-discord", "markdown quoted as written (default)", from(minimal(markdown), discord.Options{}), note("stripDiscordMarkdown defaults to false — compare with the next card")),
		quoteCase("11-discord", "stripDiscordMarkdown: true", from(minimal(markdown), discord.Options{StripMarkdown: true})),
		quoteCase("11-discord", "mentions resolved (default)", from(mentionMessage, discord.Options{}), note("resolveMentions defaults to true — compare with the next card")),
		quoteCase("11-discord", "resolveMentions: false", from(mentionMessage, discord.Options{ResolveMentions: pointer(false)})),
		quoteCase("11-discord", "slash commands, timestamps and navigation tabs", from(minimal("</remind set:1> を <t:1618935630:F> に実行し、<id:guide> を確認します"), discord.Options{Location: time.UTC}), note("these carry everything they need in the token — no lookup involved")),
	}
}

func misskeyCases() []Case {
	from := func(noteValue misskey.Note, options misskey.Options, avatar bool) quoteBuilder {
		return func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			q, err := misskey.FromNote(noteValue, options)
			if avatar {
				q.Avatar = png(r)
			}
			return q, miq.RenderOptions{}, err
		}
	}
	noteValue := misskey.Note{Text: "$[jelly 準備完了] **次** の作業へ進みます", User: misskey.User{Username: "sample_user", Name: "Sample User"}}
	return []Case{
		quoteCase("14-misskey", "a note, MFM stripped (default)", from(noteValue, misskey.Options{}, true), note("the display name goes over the @handle, exactly as Misskey writes it")),
		quoteCase("14-misskey", "stripMfm: false", from(noteValue, misskey.Options{KeepMFM: true}, true)),
		quoteCase("14-misskey", "a remote author, quoted from a note", from(misskey.Note{Text: "別のサーバーから届いた文章も同じ形式で表示します", User: misskey.User{Username: "someone", Host: "misskey.example"}}, misskey.Options{}, false), note("@user@host, and the username stands in when there is no display name")),
	}
}

func twitterCases() []Case {
	from := func(tweetValue twitter.Tweet, avatar bool) quoteBuilder {
		return func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			q, err := twitter.FromTweet(tweetValue)
			if avatar {
				q.Avatar = png(r)
			}
			return q, miq.RenderOptions{}, err
		}
	}
	return []Case{
		quoteCase("16-twitter", "a tweet, quoted via setFromTweet", from(twitter.Tweet{Text: "A small update, shared clearly.", Author: twitter.Author{Username: "sample_account", Name: "Sample Account"}}, true), note("text goes through exactly as written — nothing here to strip or resolve")),
		quoteCase("16-twitter", "no display name — falls back to the handle", from(twitter.Tweet{Text: "A post from an account that uses its handle as the display name.", Author: twitter.Author{Username: "another_account"}}, false)),
	}
}

func colorCases() []Case {
	rgba := func(hex uint32) color.NRGBA {
		return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
	}
	custom := func(background, text, display, username, watermark color.NRGBA) miq.RenderOptions {
		return miq.RenderOptions{Theme: theme.Input{Extends: theme.Custom, Background: &background, Text: &theme.TextInput{Color: &text}, DisplayName: &theme.LabelInput{Color: &display}, Username: &theme.LabelInput{Color: &username}, Watermark: &theme.LabelInput{Color: &watermark}}}
	}
	transparent := color.NRGBA{}
	white, gray := rgba(0xffffff), rgba(0xaaaaaa)
	return []Case{
		quoteCase("12-colors", "tokyo night", configured(jaText, png, custom(rgba(0x1a1b26), rgba(0xc0caf5), rgba(0x7aa2f7), rgba(0x565f89), rgba(0x414868))), note("custom starts fully transparent; every color here is set explicitly")),
		quoteCase("12-colors", "nord", configured(jaText, png, custom(rgba(0x2e3440), rgba(0xeceff4), rgba(0x88c0d0), rgba(0x4c566a), rgba(0x434c5e))), note("the same thing written as 0xRRGGBB numbers")),
		quoteCase("12-colors", "CSS colour names and hsl()", configured(jaText, png, custom(rgba(0x191970), rgba(0xe6e6fa), color.NRGBA{255, 223, 102, 255}, rgba(0x708090), transparent)), note("strings go through the color package, so all of CSS is available")),
		quoteCase("12-colors", "solarized, as rgba arrays", configured(jaText, png, custom(rgba(0x002b36), rgba(0xfdf6e3), rgba(0x2aa198), rgba(0x586e75), rgba(0x073642)))),
		quoteCase("12-colors", "translucent background over the avatar", configured(jaText, png, miq.RenderOptions{Theme: theme.Input{Extends: theme.Custom, Layout: pointer(theme.Stacked), Background: &color.NRGBA{A: 160}, Gradient: &theme.GradientInput{Enabled: pointer(false)}, Text: &theme.TextInput{Color: &white}, DisplayName: &theme.LabelInput{Color: &white}, Username: &theme.LabelInput{Color: pointer(rgba(0xcccccc))}, Watermark: &theme.LabelInput{Color: pointer(rgba(0x888888))}}}), note("#RRGGBBAA — the avatar shows through the wash")),
		quoteCase("12-colors", "transparent background (checkerboard is the page)", configured(jaText, noAvatar, miq.RenderOptions{Theme: theme.Input{Extends: theme.Custom, Text: &theme.TextInput{Color: &white}, DisplayName: &theme.LabelInput{Color: &white}, Username: &theme.LabelInput{Color: &gray}}}), note("nothing is painted where the background would be")),
		quoteCase("12-colors", "background image", func(r *Runtime) (miq.Quote, miq.RenderOptions, error) {
			opacity := .5
			options := custom(rgba(0), white, white, rgba(0xcccccc), transparent)
			options.BackgroundImage, options.BackgroundFit, options.BackgroundOpacity = photo(r), theme.Cover, &opacity
			return attributed(r, jaText, nil), options, nil
		}, note("a photo behind the quote, dimmed by backgroundImage.opacity")),
	}
}

func conversationCases() []Case {
	return []Case{
		conversationCase("13-conversation", "dark (default) — consecutive messages from one speaker group", func(r *Runtime) ([]miq.ConversationMessage, miq.ConversationOptions, error) {
			return []miq.ConversationMessage{{Username: "sample_user", DisplayName: "Sample User", Avatar: png(r), Text: jaText}, {Username: "sample_user", DisplayName: "Sample User", Avatar: png(r), Text: "内容を確認して、次の手順へ進みます。"}, {Username: "someone", Text: english}}, miq.ConversationOptions{}, nil
		}, note("the second message shares an avatar and name with the first")),
		conversationCase("13-conversation", "light", func(r *Runtime) ([]miq.ConversationMessage, miq.ConversationOptions, error) {
			return []miq.ConversationMessage{{Username: "sample_user", Avatar: photo(r), Text: shortText}, {Username: "someone", Text: "Several messages can be arranged in one clear image."}}, miq.ConversationOptions{Theme: miq.ConversationLight}, nil
		}),
		conversationCase("13-conversation", "from real messages (setFromMessages)", func(r *Runtime) ([]miq.ConversationMessage, miq.ConversationOptions, error) {
			messages := []discord.Message{{Content: "更新しました " + r.Assets.DiscordEmoji[0], Author: discord.User{Username: "sample_user", GlobalName: "Sample User", AvatarURL: r.Assets.RemoteURL}}, {Content: "内容を確認しました", Author: discord.User{Username: "sample_user", GlobalName: "Sample User"}}}
			converted, err := discord.Conversation(messages, discord.Options{})
			return converted, miq.ConversationOptions{}, err
		}, network, note("reads content/name/avatar the same way MiQ#setFromMessage() does")),
	}
}

func validateCaseCount(cases []Case) error {
	if len(cases) != 96 {
		return fmt.Errorf("gallery registry has %d cases, want 96", len(cases))
	}
	return nil
}
