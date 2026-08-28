package discord

import "testing"

func TestStrip(t *testing.T) {
	tests := map[string]string{
		"nothing":                               "nothing",
		"**bold _and italic_**":                 "bold and italic",
		"__underline__ ||spoiler||":             "underline spoiler",
		"`**not bold**`":                        "**not bold**",
		"```js\nconst a = **b**\n```":           "const a = **b**",
		"```inline block```":                    "inline block",
		">>> all of this\nis quoted":            "all of this\nis quoted",
		"### Header":                            "Header",
		"-# fine print":                         "fine print",
		"- top\n  - nested":                     "top\n  nested",
		"3.14 is pi":                            "3.14 is pi",
		"snake_case_var":                        "snakecasevar",
		"[**bold** label](https://example.com)": "bold label",
		"\\*not italic\\*":                      "*not italic*",
		"\\_still not italic_":                  "_still not italic_",
		"a `**b**` c **d**":                     "a **b** c d",
		"`**a**` then `~~b~~`":                  "**a** then ~~b~~",
		"`\\*x\\*`":                             "*x*",
		"# Header\n## Two\n### Three":           "Header\nTwo\nThree",
		"1. first\n12. twelfth":                 "first\ntwelfth",
		"12:30:45":                              "12:30:45",
		"normal *emphasis* here":                "normal emphasis here",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := Strip(input); got != want {
				t.Fatalf("Strip() = %q, want %q", got, want)
			}
		})
	}
}
