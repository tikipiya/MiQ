package mfm

import "testing"

func TestStrip(t *testing.T) {
	tests := map[string]string{
		"nothing":        "nothing",
		"$[jelly hello]": "hello",
		"$[fg.color=f00 **bold** and $[flip nested]]": "bold and nested",
		"$[unclosed":                       "$[unclosed",
		"<b>bold</b> <small>x</small>":     "bold x",
		"<center>middle</center>":          "middle",
		"a<center>b</center>":              "a<center>b</center>",
		"?[silent](https://misskey.io)":    "silent",
		"`$[jelly x]`":                     "$[jelly x]",
		"<plain>$[jelly x]</plain>":        "$[jelly x]",
		`\(x^2\)`:                          "x^2",
		"\\[\nx+y\n\\]":                    "x+y",
		"```go\n$[jelly x]\n```":           "$[jelly x]",
		"<https://misskey.io>":             "https://misskey.io",
		"> quoted\nnext":                   "quoted\nnext",
		":blobcat: @user@host #tag":        ":blobcat: @user@host #tag",
		"$[shake.speed=1s hello]":          "hello",
		"$[spin $[flip x]]":                "x",
		"a $[x b] c":                       "a b c",
		"<i>x</i> <s>y</s>":                "x y",
		"**bold** __also__ *italic* ~~s~~": "bold also italic s",
		"[Misskey](https://misskey.io)":    "Misskey",
		"a\n<center>b</center>":            "a\nb",
		"one\ntwo":                         "one\ntwo",
		"MFM https://misskey.io here":      "MFM https://misskey.io here",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := Strip(input); got != want {
				t.Fatalf("Strip() = %q, want %q", got, want)
			}
		})
	}
}
