// Package relnotes turns the release bodies on GitHub into readable terminal text.
package relnotes

import (
	"regexp"
	"strings"
)

// Release is one published release: its tag and the body written on its page.
type Release struct {
	Tag  string `json:"tag_name"`
	Body string `json:"body"`
}

// Line is one line of rendered notes; a heading is a section name such as "New".
type Line struct {
	Text    string
	Heading bool
}

var (
	heading = regexp.MustCompile(`^#+\s*(.*)$`)
	// goreleaser's auto-generated bullets: "* <hash>: <subject> (@author)" or "* <subject> by @author in <url>".
	commit = regexp.MustCompile(`^[*-]\s+(?:[0-9a-f]{7,40}:\s*)?(.*?)(?:\s+\(@[^)]+\)|\s+by @\S+ in \S+)?$`)
	bullet = regexp.MustCompile(`^[*-]\s+(.*)$`)
	// conventional commit prefixes, dropped so the note reads as a sentence.
	prefix   = regexp.MustCompile(`^(feat|fix|chore|docs|refactor|test|perf|build|ci|style)(\([^)]*\))?!?:\s*`)
	fullLog  = regexp.MustCompile(`^\*\*Full Changelog`)
	emphasis = strings.NewReplacer("**", "", "`", "")
)

// Lines renders a release body: headings become Heading lines, bullets become "•", commit hashes,
// authors, conventional prefixes and markdown emphasis go, and blank runs collapse.
func Lines(body string) []Line {
	var out []Line
	for _, raw := range strings.Split(body, "\n") {
		s := strings.TrimSpace(raw)
		switch {
		case s == "" || fullLog.MatchString(s):
			if n := len(out); n > 0 && out[n-1].Text != "" {
				out = append(out, Line{})
			}
		case heading.MatchString(s):
			out = append(out, Line{Text: strings.TrimSpace(heading.FindStringSubmatch(s)[1]), Heading: true})
		case commit.MatchString(s) && s != "":
			out = append(out, Line{Text: "• " + clean(commit.FindStringSubmatch(s)[1])})
		case bullet.MatchString(s):
			out = append(out, Line{Text: "• " + clean(bullet.FindStringSubmatch(s)[1])})
		default:
			out = append(out, Line{Text: clean(s)})
		}
	}
	for len(out) > 0 && out[len(out)-1].Text == "" {
		out = out[:len(out)-1]
	}
	return out
}

// clean drops the conventional prefix, capitalises the rest and strips markdown emphasis.
func clean(s string) string {
	s = emphasis.Replace(s)
	if p := prefix.FindString(s); p != "" {
		s = strings.TrimPrefix(s, p)
		if s != "" {
			s = strings.ToUpper(s[:1]) + s[1:]
		}
	}
	return s
}

// Text is a release as plain terminal text: the tag, then its notes indented under it.
func Text(r Release) string {
	var b strings.Builder
	b.WriteString(r.Tag + "\n")
	for _, l := range Lines(r.Body) {
		switch {
		case l.Heading:
			b.WriteString("\n  " + l.Text + "\n")
		case l.Text == "":
			b.WriteString("\n")
		default:
			b.WriteString("  " + l.Text + "\n")
		}
	}
	return b.String()
}
