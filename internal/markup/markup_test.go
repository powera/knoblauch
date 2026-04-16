package markup

import (
	"strings"
	"testing"
)

func TestRenderBodyEscapes(t *testing.T) {
	got := string(RenderBody("<script>alert(1)</script>"))
	if strings.Contains(got, "<script>") {
		t.Fatalf("unescaped script tag: %q", got)
	}
}

func TestRenderBodyBullets(t *testing.T) {
	got := string(RenderBody("* a\n* b"))
	want := "<ul><li>a</li><li>b</li></ul>"
	if got != want {
		t.Fatalf("bullets: got %q want %q", got, want)
	}
}

func TestRenderBodyAudioSentinel(t *testing.T) {
	got := string(RenderBody("header\n::audio::abc_DEF-123.xyz"))
	want := `header<br><audio controls preload="none" src="/audio/abc_DEF-123.xyz"></audio>`
	if got != want {
		t.Fatalf("audio: got %q want %q", got, want)
	}
}

func TestRenderBodyAudioBadTokenIsEscaped(t *testing.T) {
	// Token contains '<' — must fall through to plain-text escape, not an <audio> tag.
	in := "::audio::<script>"
	got := string(RenderBody(in))
	if strings.Contains(got, "<audio") {
		t.Fatalf("bad token produced audio tag: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("bad token not escaped: %q", got)
	}
}

func TestRenderBodyAudioMustBeWholeLine(t *testing.T) {
	// The sentinel must be a full line — inline occurrences are escaped.
	got := string(RenderBody("see ::audio::abc for a player"))
	if strings.Contains(got, "<audio") {
		t.Fatalf("inline sentinel produced audio tag: %q", got)
	}
}
