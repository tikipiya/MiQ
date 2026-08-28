package discord

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	maskedLink = regexp.MustCompile(`\[([^\]]+)\]\((?:[^()\\]|\\.|\([^)]*\))+\)`)
	linePrefix = regexp.MustCompile(`^(\s*)(?:#{1,3}\s+|>>>\s?|>\s?|-#\s+|[-*]\s+|\d+\.\s+)`)
	paired     = []*regexp.Regexp{
		regexp.MustCompile(`(?s)\*\*\*(.+?)\*\*\*`),
		regexp.MustCompile(`(?s)\*\*(.+?)\*\*`),
		regexp.MustCompile(`(?s)__(.+?)__`),
		regexp.MustCompile(`(?s)~~(.+?)~~`),
		regexp.MustCompile(`(?s)\|\|(.+?)\|\|`),
		regexp.MustCompile(`(?s)\*([^*\n]+?)\*`),
		regexp.MustCompile(`(?s)_([^_\n]+?)_`),
	}
)

func Strip(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	protected := make([]string, 0)
	value = protectCode(value, &protected)
	value = protectEscapes(value, &protected)

	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = linePrefix.ReplaceAllString(line, "$1")
	}
	value = strings.Join(lines, "\n")
	for {
		next := maskedLink.ReplaceAllString(value, "$1")
		if next == value {
			break
		}
		value = next
	}
	for {
		before := value
		for _, pattern := range paired {
			value = pattern.ReplaceAllString(value, "$1")
		}
		if value == before {
			break
		}
	}
	for i := len(protected) - 1; i >= 0; i-- {
		value = strings.ReplaceAll(value, token(i), protected[i])
	}
	return strings.TrimSpace(value)
}

func protectCode(value string, protected *[]string) string {
	var out strings.Builder
	for len(value) > 0 {
		if strings.HasPrefix(value, "```") {
			if end := strings.Index(value[3:], "```"); end >= 0 {
				content := value[3 : 3+end]
				if newline := strings.IndexByte(content, '\n'); newline >= 0 {
					first := content[:newline]
					if first == "" || !strings.ContainsAny(first, " \t") {
						content = content[newline+1:]
					}
				}
				content = strings.TrimSuffix(content, "\n")
				out.WriteString(stash(unescape(content), protected))
				value = value[3+end+3:]
				continue
			}
		}
		if value[0] == '`' {
			if end := strings.IndexByte(value[1:], '`'); end >= 0 {
				out.WriteString(stash(unescape(value[1:1+end]), protected))
				value = value[1+end+1:]
				continue
			}
		}
		out.WriteByte(value[0])
		value = value[1:]
	}
	return out.String()
}

func protectEscapes(value string, protected *[]string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			out.WriteString(stash(value[i+1:i+2], protected))
			i++
		} else {
			out.WriteByte(value[i])
		}
	}
	return out.String()
}

func unescape(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			i++
		}
		out.WriteByte(value[i])
	}
	return out.String()
}

func stash(value string, protected *[]string) string {
	index := len(*protected)
	*protected = append(*protected, value)
	return token(index)
}

func token(index int) string { return fmt.Sprintf("\x00MIQ%d\x00", index) }
