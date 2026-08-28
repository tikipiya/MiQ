package discord

import (
	"errors"
	"fmt"
	miq "github.com/tikipiya/MiQ"
	"strings"
	"testing"
	"time"
)

func TestFromMessage(t *testing.T) {
	resolve := true
	quote, err := FromMessage(Message{
		Content:  "**hi** <@1> in <#2> </wave:3>",
		Author:   User{Username: "cat", GlobalName: "Cat", Discriminator: "0", AvatarURL: "https://cdn.example/a.png"},
		Member:   &Member{Nickname: "Kitty", AvatarURL: "https://cdn.example/g.png"},
		Mentions: Mentions{Users: map[string]User{"1": {Username: "dog"}}, Channels: map[string]Named{"2": {Name: "general"}}},
	}, Options{StripMarkdown: true, ResolveMentions: &resolve})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Text != "hi @dog in #general /wave" || quote.DisplayName != "Kitty" || quote.Username != "cat" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestSourceCompatibilityCases(t *testing.T) {
	legacy, err := FromMessage(Message{Content: "hi", Author: User{Username: "cat", Discriminator: "1234"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Username != "cat#1234" || legacy.DisplayName != "cat" {
		t.Fatalf("legacy=%#v", legacy)
	}
	global := Options{PreferGlobalName: true}
	quote, err := FromMessage(Message{Content: "hi", Author: User{Username: "cat", GlobalName: "Global"}, Member: &Member{Nickname: "Guild"}}, global)
	if err != nil {
		t.Fatal(err)
	}
	if quote.DisplayName != "Global" {
		t.Fatalf("name=%q", quote.DisplayName)
	}
	noResolve := false
	raw, err := FromMessage(Message{Content: "**hi** <@1>", Author: User{Username: "cat"}, Mentions: Mentions{Users: map[string]User{"1": {Username: "dog"}}}}, Options{ResolveMentions: &noResolve})
	if err != nil {
		t.Fatal(err)
	}
	if raw.Text != "**hi** <@1>" {
		t.Fatalf("text=%q", raw.Text)
	}
	if _, err := FromMessage(Message{}, Options{}); !errors.Is(err, miq.ErrValidation) {
		t.Fatalf("error=%v", err)
	}
}

func TestResolveAllDiscordTokens(t *testing.T) {
	mentions := Mentions{Members: map[string]MentionedMember{"1": {Nickname: "Kitty", DisplayName: "Cat"}}, Users: map[string]User{"2": {Username: "dog"}}, Roles: map[string]Named{"3": {Name: "mods"}}, Channels: map[string]Named{"4": {Name: "general"}}}
	input := "<@1> <@!2> <@&3> <#4> </config set:5> <id:browse> <id:linked-roles> <@404>"
	want := "@Kitty @dog @mods #general /config set Browse Channels Linked Roles <@404>"
	if got := ResolveMentions(input, mentions, Options{}); got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
	const at int64 = 1618935630
	styles := map[string]string{"t": "16:20", "T": "16:20:30", "d": "20/04/2021", "D": "20 April 2021", "F": "Tuesday, 20 April 2021 16:20"}
	for style, want := range styles {
		token := "<t:" + fmt.Sprint(at) + ":" + style + ">"
		if got := ResolveMentions(token, Mentions{}, Options{Location: time.UTC}); got != want {
			t.Fatalf("%s=%q want %q", style, got, want)
		}
	}
	unknown := "<t:1618935630:Z>"
	if got := ResolveMentions(unknown, Mentions{}, Options{}); got != unknown {
		t.Fatalf("unknown=%q", got)
	}
	future := ResolveMentions("<t:1618935630:R>", Mentions{}, Options{Now: time.Unix(at-3600, 0)})
	if future != "in 1 hour" {
		t.Fatalf("future=%q", future)
	}
	past := ResolveMentions("<t:1618935630:R>", Mentions{}, Options{Now: time.Unix(at+70*86400, 0)})
	if !strings.Contains(past, "months ago") {
		t.Fatalf("past=%q", past)
	}
}

func TestConversationAdapter(t *testing.T) {
	messages, err := Conversation([]Message{{Content: "one", Author: User{Username: "a"}}, {Content: "two", Author: User{Username: "b", GlobalName: "B"}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].DisplayName != "B" {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestTimestamp(t *testing.T) {
	got := ResolveMentions("<t:0:R>", Mentions{}, Options{Now: time.Unix(60, 0)})
	if got != "1 minute ago" {
		t.Fatalf("got %q", got)
	}
}
