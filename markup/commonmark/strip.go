package commonmark

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	gmtext "github.com/yuin/goldmark/text"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func Strip(value string) string {
	source := []byte(value)
	doc := markdown.Parser().Parse(gmtext.NewReader(source))
	var out strings.Builder
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n := node.(type) {
			case *ast.Text:
				out.Write(n.Segment.Value(source))
				if n.SoftLineBreak() || n.HardLineBreak() {
					out.WriteByte('\n')
				}
			case *ast.String:
				out.Write(n.Value)
			case *ast.AutoLink:
				out.Write(n.Label(source))
				return ast.WalkSkipChildren, nil
			case *ast.CodeBlock:
				out.Write(bytes.TrimSuffix(n.Lines().Value(source), []byte("\n")))
				out.WriteString("\n\n")
				return ast.WalkSkipChildren, nil
			case *ast.FencedCodeBlock:
				out.Write(bytes.TrimSuffix(n.Lines().Value(source), []byte("\n")))
				out.WriteString("\n\n")
				return ast.WalkSkipChildren, nil
			case *ast.RawHTML:
				return ast.WalkSkipChildren, nil
			case *ast.HTMLBlock:
				return ast.WalkSkipChildren, nil
			case *ext.TaskCheckBox:
				if n.IsChecked {
					out.WriteString("[x] ")
				} else {
					out.WriteString("[ ] ")
				}
			}
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ext.TableCell:
			if n.NextSibling() != nil {
				out.WriteByte('\t')
			}
		case *ext.TableRow, *ext.TableHeader:
			out.WriteByte('\n')
		case *ast.ListItem:
			ensureNewline(&out, 1)
		case *ast.Paragraph, *ast.Heading, *ast.Blockquote:
			if _, inList := node.Parent().(*ast.ListItem); inList {
				ensureNewline(&out, 1)
			} else {
				ensureNewline(&out, 2)
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(out.String())
}

func ensureNewline(out *strings.Builder, count int) {
	value := out.String()
	trailing := 0
	for i := len(value) - 1; i >= 0 && value[i] == '\n'; i-- {
		trailing++
	}
	for trailing < count {
		out.WriteByte('\n')
		trailing++
	}
}
