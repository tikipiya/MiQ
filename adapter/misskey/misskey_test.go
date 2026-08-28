package misskey

import (
	"errors"
	miq "github.com/tikipiya/MiQ"
	"testing"
)

func TestFromNote(t *testing.T) {
	quote, err := FromNote(Note{Text: "$[jelly hello] :cat:", User: User{Username: "u", Host: "example", Name: "User"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Text != "hello :cat:" || quote.Username != "u@example" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestNoteOptionsAndFallbacks(t *testing.T) {
	cw, err := FromNote(Note{Text: "body", CW: "warning", User: User{Username: "cat"}}, Options{PreferCW: true})
	if err != nil {
		t.Fatal(err)
	}
	if cw.Text != "warning" || cw.DisplayName != "cat" {
		t.Fatalf("cw=%#v", cw)
	}
	raw, err := FromNote(Note{Text: "$[jelly body]", User: User{Username: "cat"}}, Options{KeepMFM: true})
	if err != nil {
		t.Fatal(err)
	}
	if raw.Text != "$[jelly body]" {
		t.Fatalf("raw=%q", raw.Text)
	}
	fallback, err := FromNote(Note{CW: "only cw", User: User{Username: "cat"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Text != "only cw" {
		t.Fatalf("fallback=%q", fallback.Text)
	}
	if _, err := FromNote(Note{}, Options{}); !errors.Is(err, miq.ErrValidation) {
		t.Fatalf("error=%v", err)
	}
}

func TestConversationNotes(t *testing.T) {
	messages, err := Conversation([]Note{{Text: "one", User: User{Username: "a"}}, {Text: "two", User: User{Username: "b", Name: "Bee"}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].DisplayName != "Bee" {
		t.Fatalf("messages=%#v", messages)
	}
}
