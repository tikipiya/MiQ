// Package mfm converts Misskey-flavoured markup to the text visible to a user.
package mfm

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	linkPattern   = regexp.MustCompile(`\??\[([^\]]+)\]\((?:[^()\\]|\\.|\([^)]*\))+\)`)
	autolink      = regexp.MustCompile(`<((?:https?://)[^>]+)>`)
	inlineTag     = regexp.MustCompile(`(?is)</?(?:b|i|s|small)>`)
	pairedMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?s)\*\*(.+?)\*\*`),
		regexp.MustCompile(`(?s)__(.+?)__`),
		regexp.MustCompile(`(?s)~~(.+?)~~`),
		regexp.MustCompile(`(?s)\*([^*\n]+?)\*`),
	}
)

// Strip removes MFM presentation syntax while retaining emoji codes, mentions,
// hashtags, URLs, and the verbatim contents of code, math, and <plain> nodes.
func Strip(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	protected := make([]string, 0)
	value = protectVerbatim(value, &protected)
	value = stripFunctions(value)
	value = stripCenters(value)
	value = inlineTag.ReplaceAllString(value, "")
	value = linkPattern.ReplaceAllString(value, "$1")
	value = autolink.ReplaceAllString(value, "$1")

	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "> ") {
			lines[i] = line[2:]
		}
	}
	value = strings.Join(lines, "\n")
	for {
		before := value
		for _, marker := range pairedMarkers {
			value = marker.ReplaceAllString(value, "$1")
		}
		if value == before {
			break
		}
	}
	for i := len(protected) - 1; i >= 0; i-- {
		value = strings.ReplaceAll(value, mfmToken(i), protected[i])
	}
	return strings.TrimSpace(value)
}

func stripFunctions(value string) string {
	for start := strings.LastIndex(value, "$["); start >= 0; start = strings.LastIndex(value[:start], "$[") {
		depth, end := 0, -1
		for i := start + 2; i < len(value); i++ {
			switch value[i] {
			case '[':
				depth++
			case ']':
				if depth == 0 {
					end = i
					i = len(value)
				} else {
					depth--
				}
			}
		}
		if end < 0 {
			continue
		}
		body := value[start+2 : end]
		if space := strings.IndexByte(body, ' '); space >= 0 {
			body = body[space+1:]
		} else {
			continue
		}
		value = value[:start] + body + value[end+1:]
	}
	return value
}

func stripCenters(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "<center>") && strings.HasSuffix(strings.ToLower(line), "</center>") {
			lines[i] = line[len("<center>") : len(line)-len("</center>")]
		}
	}
	return strings.Join(lines, "\n")
}

func protectVerbatim(value string, protected *[]string) string {
	var out strings.Builder
	for len(value) > 0 {
		lower := strings.ToLower(value)
		if strings.HasPrefix(lower, "<plain>") {
			if end := strings.Index(lower[len("<plain>"):], "</plain>"); end >= 0 {
				content := value[len("<plain>") : len("<plain>")+end]
				out.WriteString(stashMFM(content, protected))
				value = value[len("<plain>")+end+len("</plain>"):]
				continue
			}
		}
		if strings.HasPrefix(value, "```") {
			if end := strings.Index(value[3:], "```"); end >= 0 {
				content := strings.Trim(value[3:3+end], "\n")
				if newline := strings.IndexByte(content, '\n'); newline >= 0 && !strings.ContainsAny(content[:newline], " \t") {
					content = content[newline+1:]
				}
				out.WriteString(stashMFM(content, protected))
				value = value[3+end+3:]
				continue
			}
		}
		if strings.HasPrefix(value, "`") {
			if end := strings.Index(value[1:], "`"); end >= 0 {
				out.WriteString(stashMFM(value[1:1+end], protected))
				value = value[1+end+1:]
				continue
			}
		}
		if strings.HasPrefix(value, `\(`) {
			if end := strings.Index(value[2:], `\)`); end >= 0 {
				out.WriteString(stashMFM(value[2:2+end], protected))
				value = value[2+end+2:]
				continue
			}
		}
		if strings.HasPrefix(value, `\[`) {
			if end := strings.Index(value[2:], `\]`); end >= 0 {
				out.WriteString(stashMFM(strings.Trim(value[2:2+end], "\n"), protected))
				value = value[2+end+2:]
				continue
			}
		}
		out.WriteByte(value[0])
		value = value[1:]
	}
	return out.String()
}

func stashMFM(value string, protected *[]string) string {
	index := len(*protected)
	*protected = append(*protected, value)
	return mfmToken(index)
}

func mfmToken(index int) string { return fmt.Sprintf("\x00MFM%d\x00", index) }
