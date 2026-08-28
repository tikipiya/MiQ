package emoji

import (
	"reflect"
	"testing"
)

func TestSegmentTextRecognizesTwemojiZWJ(t *testing.T) {
	segments := SegmentText("A👨‍👩‍👧‍👦B", SegmentOptions{})
	if len(segments) != 3 || segments[1].Source != Twemoji {
		t.Fatalf("segments = %#v", segments)
	}
	if got, want := segments[1].URL, twemojiBase+"1f468-200d-1f469-200d-1f467-200d-1f466.png"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestSegmentTextRecognizesDiscord(t *testing.T) {
	segments := SegmentText("x <:blob:12345678901234567> y", SegmentOptions{})
	if len(segments) != 3 || segments[1].Source != Discord || segments[1].Name != "blob" {
		t.Fatalf("segments = %#v", segments)
	}
	if got := segments[1].URL; got != "https://cdn.discordapp.com/emojis/12345678901234567.png?size=64" {
		t.Fatalf("URL = %q", got)
	}
}

func TestSegmentTextMisskeyResolution(t *testing.T) {
	segments := SegmentText(":blob: :remote@example.com:", SegmentOptions{
		Misskey: MisskeyOptions{Instances: []string{"https://misskey.local"}},
	})
	if len(segments) != 3 || segments[0].Source != Misskey || segments[2].Host != "example.com" {
		t.Fatalf("segments = %#v", segments)
	}
	remote := false
	got := SegmentText(":remote@example.com:", SegmentOptions{Misskey: MisskeyOptions{Remote: &remote}})
	if len(got) != 1 || got[0].Text != ":remote@example.com:" {
		t.Fatalf("remote-disabled segments = %#v", got)
	}
}

func TestSegmentTextDoesNotTreatTimeAsMisskey(t *testing.T) {
	got := SegmentText("12:30:45", SegmentOptions{Misskey: MisskeyOptions{Instances: []string{"example.com"}}})
	want := []Segment{{Text: "12:30:45"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %#v", got)
	}
}

func TestResolveMissing(t *testing.T) {
	segments := []Segment{{Text: "a"}, {Source: Twemoji, URL: "missing", Raw: "😀"}}
	got, err := ResolveMissing(segments, func(string) bool { return false }, "text")
	if err != nil || len(got) != 1 || got[0].Text != "a😀" {
		t.Fatalf("resolved = %#v, err=%v", got, err)
	}
	if _, err := ResolveMissing(segments, func(string) bool { return false }, "throw"); err == nil {
		t.Fatal("expected missing emoji error")
	}
}
