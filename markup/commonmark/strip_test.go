package commonmark

import "testing"

func TestStrip(t *testing.T) {
	tests := map[string]string{
		"nothing":                                    "nothing",
		"**bold _and italic_**":                      "bold and italic",
		"`**literal**`":                              "**literal**",
		"```js\nconst a = 1\n```":                    "const a = 1",
		"# Heading":                                  "Heading",
		"> para one\n>\n> para two":                  "para one\n\npara two",
		"[a link](https://example.com)":              "a link",
		"![alt text](https://example.com/image.png)": "alt text",
		"see <https://example.com> here":             "see https://example.com here",
		"- item one\n- item two":                     "item one\nitem two",
		"- [ ] todo\n- [x] done":                     "[ ] todo\n[x] done",
		"line one\nwith a break  \nhard break":       "line one\nwith a break\nhard break",
		"before\n\n---\n\nafter":                     "before\n\nafter",
		"| a | b |\n| --- | --- |\n| 1 | 2 |":        "a\tb\n1\t2",
		"text with <em>raw html</em> inline":         "text with raw html inline",
		"<div>\nraw html block\n</div>\n\nafter":     "after",
		"[label][ref]\n\n[ref]: https://example.com": "label",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := Strip(input); got != want {
				t.Fatalf("Strip() = %q, want %q", got, want)
			}
		})
	}
}
