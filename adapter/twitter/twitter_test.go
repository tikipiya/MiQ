package twitter

import (
	"errors"
	miq "github.com/tikipiya/MiQ"
	"testing"
)

func TestFromAPIV2(t *testing.T) {
	quote, err := FromAPIV2(APIV2Tweet{Text: "hello", AuthorID: "1"}, []APIV2User{{ID: "1", Username: "cat", Name: "Cat"}})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Text != "hello" || quote.Username != "cat" || quote.DisplayName != "Cat" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestTwitterAdapters(t *testing.T) {
	fx, err := FromFxStatus(FxStatus{Text: "hello", ScreenName: "cat", Name: "Cat"})
	if err != nil {
		t.Fatal(err)
	}
	if fx.Text != "hello" || fx.Username != "cat" || fx.DisplayName != "Cat" {
		t.Fatalf("fx=%#v", fx)
	}
	if _, err := FromAPIV2(APIV2Tweet{Text: "missing", AuthorID: "404"}, nil); !errors.Is(err, miq.ErrValidation) {
		t.Fatalf("error=%v", err)
	}
	if _, err := FromTweet(Tweet{}); !errors.Is(err, miq.ErrValidation) {
		t.Fatalf("error=%v", err)
	}
}

func TestConversationTweets(t *testing.T) {
	messages, err := Conversation([]Tweet{{Text: "one", Author: Author{Username: "a"}}, {Text: "two", Author: Author{Username: "b", Name: "Bee"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].DisplayName != "Bee" {
		t.Fatalf("messages=%#v", messages)
	}
}
