// Package markup converts plain-text message bodies to safe HTML.
// The only supported constructs are:
//   - Lines starting with "* " → bullet list items (<ul><li>…</li></ul>)
//   - All other newlines → <br>
//
// All text content is HTML-escaped before any tags are inserted, so the
// output is safe to inject as innerHTML.
package markup

import (
	"html"
	"html/template"
	"strings"
)

// RenderBody converts a plain-text message body to safe HTML.
func RenderBody(body string) template.HTML {
	lines := strings.Split(body, "\n")

	var out strings.Builder
	inList := false

	flushList := func() {
		if inList {
			out.WriteString("</ul>")
			inList = false
		}
	}

	for i, line := range lines {
		if strings.HasPrefix(line, "* ") {
			// Bullet list item.
			if !inList {
				out.WriteString("<ul>")
				inList = true
			}
			out.WriteString("<li>")
			out.WriteString(html.EscapeString(line[2:]))
			out.WriteString("</li>")
		} else {
			flushList()
			if i > 0 {
				out.WriteString("<br>")
			}
			out.WriteString(html.EscapeString(line))
		}
	}
	flushList()

	return template.HTML(out.String())
}
