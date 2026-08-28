// Package discord adapts Discord messages into makeitaquote inputs.
package discord

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	miq "github.com/tikipiya/MiQ"
	discordmarkup "github.com/tikipiya/MiQ/markup/discord"
)

type User struct {
	Username, GlobalName, Discriminator, AvatarURL string
}

type Member struct {
	Nickname, DisplayName, AvatarURL string
}

type Named struct{ Name string }
type MentionedMember struct{ Nickname, DisplayName string }
type Mentions struct {
	Members  map[string]MentionedMember
	Users    map[string]User
	Roles    map[string]Named
	Channels map[string]Named
}

type Message struct {
	Content  string
	Author   User
	Member   *Member
	Mentions Mentions
}

type Options struct {
	PreferGlobalAvatar bool
	PreferGlobalName   bool
	StripMarkdown      bool
	ResolveMentions    *bool
	Location           *time.Location
	Now                time.Time
}

var (
	userMention    = regexp.MustCompile(`<@!?(\d+)>`)
	roleMention    = regexp.MustCompile(`<@&(\d+)>`)
	channelMention = regexp.MustCompile(`<#(\d+)>`)
	commandMention = regexp.MustCompile(`</([\w-]+(?: [\w-]+){0,2}):\d+>`)
	timestamp      = regexp.MustCompile(`<t:(-?\d+)(?::([tTdDfFsR]))?>`)
	navigation     = regexp.MustCompile(`<id:([a-z-]+)>`)
)

func FromMessage(message Message, options Options) (miq.Quote, error) {
	if message.Author.Username == "" {
		return miq.Quote{}, &miq.FieldError{Field: "message.author.username", Err: miq.ErrValidation}
	}
	text := message.Content
	if options.ResolveMentions == nil || *options.ResolveMentions {
		text = ResolveMentions(text, message.Mentions, options)
	}
	if options.StripMarkdown {
		text = discordmarkup.Strip(text)
	}
	username := message.Author.Username
	if message.Author.Discriminator != "" && message.Author.Discriminator != "0" {
		username += "#" + message.Author.Discriminator
	}
	guildName, guildAvatar := "", ""
	if message.Member != nil {
		guildName, guildAvatar = message.Member.Nickname, message.Member.AvatarURL
		if guildName == "" {
			guildName = message.Member.DisplayName
		}
	}
	displayName := first(guildName, message.Author.GlobalName, message.Author.Username)
	avatar := first(guildAvatar, message.Author.AvatarURL)
	if options.PreferGlobalName {
		displayName = first(message.Author.GlobalName, guildName, message.Author.Username)
	}
	if options.PreferGlobalAvatar {
		avatar = first(message.Author.AvatarURL, guildAvatar)
	}
	quote := miq.Quote{Text: text, Username: username, DisplayName: displayName}
	if avatar != "" {
		parsed, err := url.ParseRequestURI(avatar)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return miq.Quote{}, &miq.FieldError{Field: "message.avatar", Err: fmt.Errorf("invalid URL: %w", miq.ErrValidation)}
		}
		quote.Avatar = miq.ImageURL(parsed)
	}
	return quote, nil
}

func Conversation(messages []Message, options Options) ([]miq.ConversationMessage, error) {
	out := make([]miq.ConversationMessage, len(messages))
	for i, message := range messages {
		quote, err := FromMessage(message, options)
		if err != nil {
			return nil, fmt.Errorf("messages[%d]: %w", i, err)
		}
		out[i] = miq.ConversationMessage{Text: quote.Text, Username: quote.Username, DisplayName: quote.DisplayName, Avatar: quote.Avatar}
	}
	return out, nil
}

func ResolveMentions(text string, mentions Mentions, options Options) string {
	text = userMention.ReplaceAllStringFunc(text, func(token string) string {
		id := userMention.FindStringSubmatch(token)[1]
		member, memberOK := mentions.Members[id]
		user, userOK := mentions.Users[id]
		if memberOK {
			if name := first(member.Nickname, member.DisplayName); name != "" {
				return "@" + name
			}
		}
		if userOK && user.Username != "" {
			return "@" + user.Username
		}
		return token
	})
	text = roleMention.ReplaceAllStringFunc(text, func(token string) string {
		if item, ok := mentions.Roles[roleMention.FindStringSubmatch(token)[1]]; ok && item.Name != "" {
			return "@" + item.Name
		}
		return token
	})
	text = channelMention.ReplaceAllStringFunc(text, func(token string) string {
		if item, ok := mentions.Channels[channelMention.FindStringSubmatch(token)[1]]; ok && item.Name != "" {
			return "#" + item.Name
		}
		return token
	})
	text = commandMention.ReplaceAllString(text, "/$1")
	labels := map[string]string{"browse": "Browse Channels", "customize": "Channels & Roles", "guide": "Server Guide", "linked-roles": "Linked Roles"}
	text = navigation.ReplaceAllStringFunc(text, func(token string) string {
		if label := labels[navigation.FindStringSubmatch(token)[1]]; label != "" {
			return label
		}
		return token
	})
	return timestamp.ReplaceAllStringFunc(text, func(token string) string {
		match := timestamp.FindStringSubmatch(token)
		var seconds int64
		if _, err := fmt.Sscan(match[1], &seconds); err != nil {
			return token
		}
		instant := time.Unix(seconds, 0)
		location := options.Location
		if location == nil {
			location = time.UTC
		}
		instant = instant.In(location)
		style := match[2]
		if style == "R" {
			now := options.Now
			if now.IsZero() {
				now = time.Now()
			}
			return relative(instant.Sub(now))
		}
		layouts := map[string]string{"t": "15:04", "T": "15:04:05", "d": "02/01/2006", "D": "2 January 2006", "f": "2 January 2006 15:04", "F": "Monday, 2 January 2006 15:04", "s": "02/01/2006 15:04"}
		layout := layouts[style]
		if layout == "" {
			layout = layouts["f"]
		}
		return instant.Format(layout)
	})
}

func relative(duration time.Duration) string {
	future := duration >= 0
	if !future {
		duration = -duration
	}
	value, unit := int64(duration.Seconds()), "second"
	for _, candidate := range []struct {
		size int64
		name string
	}{{31536000, "year"}, {2592000, "month"}, {604800, "week"}, {86400, "day"}, {3600, "hour"}, {60, "minute"}} {
		if value >= candidate.size {
			value, unit = (value+candidate.size/2)/candidate.size, candidate.name
			break
		}
	}
	if value != 1 {
		unit += "s"
	}
	if future {
		return fmt.Sprintf("in %d %s", value, unit)
	}
	return fmt.Sprintf("%d %s ago", value, unit)
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
