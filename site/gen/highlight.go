package main

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var (
	codeRe   = regexp.MustCompile(`(?s)<pre><code(?: class="language-([A-Za-z0-9_+-]+)")?>(.*?)</code></pre>`)
	assignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	shellRe  = regexp.MustCompile(`^(\$ |verd |go |git |curl |export |XDG_|gofmt |goreleaser |python3 |tmux )`)
)

// highlightCode colors fenced code blocks: shell gets comments/commands/flags/strings picked out,
// other languages go through chroma with CSS classes (styled in site.css).
func highlightCode(page string) string {
	return codeRe.ReplaceAllStringFunc(page, func(m string) string {
		sub := codeRe.FindStringSubmatch(m)
		lang, code := strings.ToLower(sub[1]), html.UnescapeString(sub[2])
		switch {
		case lang == "sh" || lang == "bash" || lang == "shell" || lang == "zsh" || lang == "console":
			return `<pre><code class="hl">` + shell(code) + `</code></pre>`
		case lang == "" && shellRe.MatchString(strings.TrimLeft(code, "\n# ")):
			return `<pre><code class="hl">` + shell(code) + `</code></pre>`
		case lang == "":
			return m
		}
		lx := lexers.Get(lang)
		if lx == nil {
			return m
		}
		it, err := chroma.Coalesce(lx).Tokenise(nil, code)
		if err != nil {
			return m
		}
		var b bytes.Buffer
		f := chtml.New(chtml.WithClasses(true), chtml.PreventSurroundingPre(true))
		if f.Format(&b, styles.Fallback, it) != nil {
			return m
		}
		return `<pre><code class="hl">` + b.String() + `</code></pre>`
	})
}

func span(class, s string) string {
	return `<span class="` + class + `">` + html.EscapeString(s) + `</span>`
}

// shell highlights one block line by line: # comments, the command word, flags, strings, operators.
func shell(code string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
		out = append(out, shellLine(line))
	}
	return strings.Join(out, "\n")
}

func shellLine(line string) string {
	body, comment := splitComment(line)
	var b strings.Builder
	cmd := true // the next word is a command
	rest := body
	if strings.HasPrefix(strings.TrimLeft(rest, " "), "$ ") {
		i := strings.Index(rest, "$ ")
		b.WriteString(rest[:i] + span("sp", "$ "))
		rest = rest[i+2:]
	}
	for len(rest) > 0 {
		if rest[0] == ' ' || rest[0] == '\t' {
			j := 0
			for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
				j++
			}
			b.WriteString(rest[:j])
			rest = rest[j:]
			continue
		}
		j := wordEnd(rest)
		w := rest[:j]
		rest = rest[j:]
		var h string
		h, cmd = shellWord(w, cmd)
		b.WriteString(h)
	}
	if comment != "" {
		b.WriteString(span("cm", comment))
	}
	return b.String()
}

// splitComment splits a shell line at its trailing comment: a '#' outside quotes that starts a word.
func splitComment(line string) (body, comment string) {
	body = line
	inS, inD := false, false
	for i, r := range line {
		switch {
		case r == '\'' && !inD:
			inS = !inS
		case r == '"' && !inS:
			inD = !inD
		case r == '#' && !inS && !inD && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			body, comment = line[:i], line[i:]
		}
		if comment != "" {
			break
		}
	}
	return body, comment
}

// wordEnd is the length of the word at the start of s, keeping quoted parts together.
func wordEnd(s string) int {
	j, q := 0, byte(0)
	for j < len(s) {
		c := s[j]
		if q != 0 {
			if c == q {
				q = 0
			}
		} else if c == '\'' || c == '"' {
			q = c
		} else if c == ' ' || c == '\t' {
			break
		}
		j++
	}
	return j
}

// shellWord is the highlighted form of one word, and whether the word after it is a command.
func shellWord(w string, cmd bool) (string, bool) {
	switch {
	case w == "|" || w == "&&" || w == ";" || w == ">" || w == ">>" || w == "<" || w == "||":
		return span("so", w), w == "|" || w == "&&" || w == ";" || w == "||"
	case strings.HasPrefix(w, "-") && len(w) > 1:
		return span("sf", w), cmd
	case strings.ContainsAny(w[:1], `'"`) || strings.Contains(w, `"`) && strings.Contains(w, "="):
		return span("ss", w), cmd
	case assignRe.MatchString(w):
		name, val, _ := strings.Cut(w, "=")
		return span("sv", name+"=") + span("ss", val), cmd
	case cmd:
		return span("sc", w), false
	case strings.HasPrefix(w, "$"):
		return span("sv", w), cmd
	}
	return html.EscapeString(w), cmd
}
