// Package misskey adapts Misskey notes into makeitaquote inputs.
package misskey

import (
	"fmt"
	"net/url"

	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/markup/mfm"
)

type User struct{ Username, Name, Host, AvatarURL string }
type Note struct {
	Text, CW string
	User     User
}
type Options struct{ KeepMFM, PreferCW bool }

func FromNote(note Note, options Options) (miq.Quote, error) {
	if note.User.Username == "" {
		return miq.Quote{}, &miq.FieldError{Field: "note.user.username", Err: miq.ErrValidation}
	}
	text := note.Text
	if options.PreferCW && note.CW != "" {
		text = note.CW
	} else if text == "" {
		text = note.CW
	}
	if !options.KeepMFM {
		text = mfm.Strip(text)
	}
	handle := note.User.Username
	if note.User.Host != "" {
		handle += "@" + note.User.Host
	}
	name := note.User.Name
	if name == "" {
		name = note.User.Username
	}
	quote := miq.Quote{Text: text, Username: handle, DisplayName: name}
	if note.User.AvatarURL != "" {
		parsed, err := url.ParseRequestURI(note.User.AvatarURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return miq.Quote{}, &miq.FieldError{Field: "note.user.avatarUrl", Err: fmt.Errorf("invalid URL: %w", miq.ErrValidation)}
		}
		quote.Avatar = miq.ImageURL(parsed)
	}
	return quote, nil
}

func Conversation(notes []Note, options Options) ([]miq.ConversationMessage, error) {
	out := make([]miq.ConversationMessage, len(notes))
	for i, note := range notes {
		quote, err := FromNote(note, options)
		if err != nil {
			return nil, fmt.Errorf("notes[%d]: %w", i, err)
		}
		out[i] = miq.ConversationMessage{Text: quote.Text, Username: quote.Username, DisplayName: quote.DisplayName, Avatar: quote.Avatar}
	}
	return out, nil
}
