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
	codeRe  = regexp.MustCompile(`(?s)<pre><code(?: class="language-([A-Za-z0-9_+-]+)")?>(.*?)</code></pre>`)
	shellRe = regexp.MustCompile(`^(\$ |verd |go |git |curl |export |XDG_|gofmt |goreleaser |python3 |tmux )`)
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
	body, comment := line, ""
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
		// one word, keeping quoted parts together
		j, q := 0, byte(0)
		for j < len(rest) {
			c := rest[j]
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
		w := rest[:j]
		rest = rest[j:]
		switch {
		case w == "|" || w == "&&" || w == ";" || w == ">" || w == ">>" || w == "<" || w == "||":
			b.WriteString(span("so", w))
			cmd = w == "|" || w == "&&" || w == ";" || w == "||"
		case strings.HasPrefix(w, "-") && len(w) > 1:
			b.WriteString(span("sf", w))
		case strings.ContainsAny(w[:1], `'"`) || strings.Contains(w, `"`) && strings.Contains(w, "="):
			b.WriteString(span("ss", w))
		case regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`).MatchString(w):
			name, val, _ := strings.Cut(w, "=")
			b.WriteString(span("sv", name+"=") + span("ss", val))
		case cmd:
			b.WriteString(span("sc", w))
			cmd = false
		case strings.HasPrefix(w, "$"):
			b.WriteString(span("sv", w))
		default:
			b.WriteString(html.EscapeString(w))
		}
	}
	if comment != "" {
		b.WriteString(span("cm", comment))
	}
	return b.String()
}
