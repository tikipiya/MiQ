// Package twitter adapts X/Twitter post representations into makeitaquote inputs.
package twitter

import (
	"fmt"
	"net/url"

	miq "github.com/tikipiya/MiQ"
)

type Author struct{ Username, Name, AvatarURL string }
type Tweet struct {
	Text   string
	Author Author
}

func FromTweet(tweet Tweet) (miq.Quote, error) {
	if tweet.Author.Username == "" {
		return miq.Quote{}, &miq.FieldError{Field: "tweet.author.username", Err: miq.ErrValidation}
	}
	name := tweet.Author.Name
	if name == "" {
		name = tweet.Author.Username
	}
	quote := miq.Quote{Text: tweet.Text, Username: tweet.Author.Username, DisplayName: name}
	if tweet.Author.AvatarURL != "" {
		parsed, err := url.ParseRequestURI(tweet.Author.AvatarURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return miq.Quote{}, &miq.FieldError{Field: "tweet.author.avatarUrl", Err: fmt.Errorf("invalid URL: %w", miq.ErrValidation)}
		}
		quote.Avatar = miq.ImageURL(parsed)
	}
	return quote, nil
}

func Conversation(tweets []Tweet) ([]miq.ConversationMessage, error) {
	out := make([]miq.ConversationMessage, len(tweets))
	for i, tweet := range tweets {
		quote, err := FromTweet(tweet)
		if err != nil {
			return nil, fmt.Errorf("tweets[%d]: %w", i, err)
		}
		out[i] = miq.ConversationMessage{Text: quote.Text, Username: quote.Username, DisplayName: quote.DisplayName, Avatar: quote.Avatar}
	}
	return out, nil
}

type APIV2Tweet struct{ Text, AuthorID string }
type APIV2User struct{ ID, Username, Name, ProfileImageURL string }

func FromAPIV2(tweet APIV2Tweet, users []APIV2User) (miq.Quote, error) {
	for _, user := range users {
		if user.ID == tweet.AuthorID {
			return FromTweet(Tweet{Text: tweet.Text, Author: Author{Username: user.Username, Name: user.Name, AvatarURL: user.ProfileImageURL}})
		}
	}
	return miq.Quote{}, &miq.FieldError{Field: "tweet.author_id", Err: fmt.Errorf("author not found: %w", miq.ErrValidation)}
}

type FxStatus struct{ Text, ScreenName, Name, AvatarURL string }

func FromFxStatus(status FxStatus) (miq.Quote, error) {
	return FromTweet(Tweet{Text: status.Text, Author: Author{Username: status.ScreenName, Name: status.Name, AvatarURL: status.AvatarURL}})
}
