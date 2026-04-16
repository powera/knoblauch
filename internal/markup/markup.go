// Package markup converts plain-text message bodies to safe HTML.
// The only supported constructs are:
//   - Lines starting with "* " → bullet list items (<ul><li>…</li></ul>)
//   - A whole line matching "::audio::<token>" → <audio> tag served by the
//     audio proxy. The token charset is restricted so the src attribute is
//     always a safe same-origin URL.
//   - All other newlines → <br>
//
// All text content is HTML-escaped before any tags are inserted, so the
// output is safe to inject as innerHTML.
package markup

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// audioLine matches a whole line of the form "::audio::<token>" where the
// token only contains base64-url and dot characters (as produced by
// integration.SignAudioURL). Anything outside that set falls through to the
// plain-text path and is HTML-escaped.
var audioLine = regexp.MustCompile(`^::audio::([A-Za-z0-9_\-.]+)$`)

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
			continue
		}

		if m := audioLine.FindStringSubmatch(line); m != nil {
			flushList()
			if i > 0 {
				out.WriteString("<br>")
			}
			// m[1] is guaranteed safe by the regex charset.
			out.WriteString(`<audio controls preload="none" src="/audio/`)
			out.WriteString(m[1])
			out.WriteString(`"></audio>`)
			continue
		}

		flushList()
		if i > 0 {
			out.WriteString("<br>")
		}
		out.WriteString(html.EscapeString(line))
	}
	flushList()

	return template.HTML(out.String())
}
