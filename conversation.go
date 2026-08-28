package miq

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"

	internaldraw "github.com/tikipiya/MiQ/internal/draw"
	internalemoji "github.com/tikipiya/MiQ/internal/emoji"
)

func (e *Engine) RenderConversation(ctx context.Context, messages []ConversationMessage, options ConversationOptions) (*image.NRGBA, error) {
	if ctx == nil {
		return nil, validationError("context", "must not be nil")
	}
	if len(messages) == 0 {
		return nil, validationError("messages", "must contain at least one message")
	}
	width := options.Width
	if width == 0 {
		width = 600
	}
	if width <= 0 || width > 8192 {
		return nil, validationError("width", "must be between 1 and 8192")
	}
	if options.Theme == "" {
		options.Theme = ConversationDark
	}
	if options.Theme != ConversationDark && options.Theme != ConversationLight {
		return nil, validationError("theme", "must be dark or light")
	}
	if e.autoFont {
		if err := e.fontManager.EnsureStack(ctx, "M PLUS Rounded 1c, Noto Sans JP, Yu Gothic, Meiryo, sans-serif"); err != nil && e.strictFonts {
			return nil, &FieldError{Field: "font", Err: fmt.Errorf("%v: %w", err, ErrFont)}
		}
	}
	drawn := make([]internaldraw.ConversationMessage, len(messages))
	for i, message := range messages {
		if strings.TrimSpace(message.Text) == "" {
			return nil, validationError(fmt.Sprintf("messages[%d].text", i), "must not be empty")
		}
		if strings.TrimSpace(message.Username) == "" {
			return nil, validationError(fmt.Sprintf("messages[%d].username", i), "must not be empty")
		}
		if utf16Length(message.Text) > MaxTextLength {
			return nil, validationError(fmt.Sprintf("messages[%d].text", i), fmt.Sprintf("must be at most %d UTF-16 code units", MaxTextLength))
		}
		if utf16Length(message.Username) > MaxNameLength {
			return nil, validationError(fmt.Sprintf("messages[%d].username", i), fmt.Sprintf("must be at most %d UTF-16 code units", MaxNameLength))
		}
		if utf16Length(message.DisplayName) > MaxNameLength {
			return nil, validationError(fmt.Sprintf("messages[%d].displayName", i), fmt.Sprintf("must be at most %d UTF-16 code units", MaxNameLength))
		}
		segments := internalemoji.SegmentText(message.Text, internalemoji.SegmentOptions{Misskey: internalemoji.MisskeyOptions{Instances: options.Misskey.Instances, Remote: options.Misskey.Remote}})
		images := e.emojiLoader.Prefetch(ctx, segments)
		behavior := string(options.OnAssetError)
		if behavior == "" {
			behavior = string(AssetAsText)
		}
		var err error
		segments, err = internalemoji.ResolveMissing(segments, func(source string) bool { return images[source] != nil }, behavior)
		if err != nil {
			return nil, &AssetError{Source: fmt.Sprintf("messages[%d].emoji", i), Err: fmt.Errorf("%v: %w", err, ErrAsset)}
		}
		avatar, err := e.resolveImage(ctx, message.Avatar)
		if err != nil {
			return nil, err
		}
		drawn[i] = internaldraw.ConversationMessage{Segments: segments, Images: images, Username: message.Username, DisplayName: message.DisplayName, Avatar: avatar}
	}
	style := internaldraw.ConversationStyle{Width: width, Background: color.NRGBA{A: 255}, Text: color.NRGBA{255, 255, 255, 255}, Name: color.NRGBA{255, 255, 255, 255}, FallbackBackground: color.NRGBA{30, 30, 30, 255}, FallbackText: color.NRGBA{255, 255, 255, 255}}
	if options.Theme == ConversationLight {
		style.Background = color.NRGBA{255, 255, 255, 255}
		style.Text = color.NRGBA{17, 17, 17, 255}
		style.Name = style.Text
		style.FallbackBackground = color.NRGBA{232, 232, 232, 255}
		style.FallbackText = style.Text
	}
	result, err := internaldraw.RenderConversation(drawn, style, e.fonts)
	if err != nil {
		return nil, fmt.Errorf("render conversation: %w: %w", err, ErrRender)
	}
	return result, nil
}

func (e *Engine) WriteConversation(ctx context.Context, w io.Writer, messages []ConversationMessage, options ConversationOptions, encode EncodeOptions) error {
	if w == nil {
		return validationError("writer", "must not be nil")
	}
	img, err := e.RenderConversation(ctx, messages, options)
	if err != nil {
		return err
	}
	return Encode(w, img, encode)
}
