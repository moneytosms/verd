// Command gen builds the verd website into -out from site/content (hand-written pages),
// docs/*.md (rendered, so the site never drifts from the docs) and site/assets.
//
//	go run ./site/gen -out _site
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

const repo = "https://github.com/moneytosms/verd"

// docOrder is the sidebar order; blurb is the card text on the docs index.
var docOrder = []struct{ file, blurb string }{
	{"customizing", "Set up and restyle verd from the CLI: shortcuts, themes, layout, reading. Written for people and agents."},
	{"config", "Every config key, languages, Templates and where verd keeps files."},
	{"keys", "Every shortcut, generated from the keymap, plus search and filter syntax."},
	{"testing", "Test Runs, Comparison Modes, Custom Tests and the stress workflow."},
	{"neovim", "Split modes and copy-pasteable keymaps for verd test, submit and stress."},
	{"embedded-pane", "Running Neovim inside verd: focus keys, sides, zoom, known gaps."},
	{"sources", "CSES next to Codeforces, the Source filter and manual solved marks."},
	{"submit", "Browser handoff, direct mode, verd login and the risks."},
}

type navItem struct {
	Title, Href, Blurb string
	On                 bool
}

type page struct {
	Title, Desc, Root, Body string
	Nav                     template.HTML
	Footer                  template.HTML
	Side                    []navItem
	SideTitle               string
	TOC                     []navItem
	Prev, Next              *navItem
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(ghtml.WithUnsafe()),
)

var (
	h1Re   = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	h2Re   = regexp.MustCompile(`<h2 id="([^"]+)">(.*?)</h2>`)
	hrefRe = regexp.MustCompile(`href="([^"#]*)(#[^"]*)?"`)
	tagRe  = regexp.MustCompile(`<[^>]+>`)
)

func main() {
	out := flag.String("out", "_site", "output directory")
	flag.Parse()
	if err := build(*out); err != nil {
		fmt.Fprintln(os.Stderr, "sitegen:", err)
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func build(out string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	must(os.RemoveAll(out))
	must(os.MkdirAll(filepath.Join(out, "docs"), 0o755))
	must(copyDir("site/assets", filepath.Join(out, "assets")))

	// hand-written home page: only the chrome placeholders are filled in
	home, err := os.ReadFile("site/content/index.html")
	must(err)
	s := strings.NewReplacer("{{ROOT}}", "", "{{NAV}}", nav("", "home"), "{{FOOTER}}", footer("")).Replace(string(home))
	must(os.WriteFile(filepath.Join(out, "index.html"), []byte(s), 0o644))

	// prose pages: install, usage
	for _, p := range []struct{ file, desc string }{
		{"install", "Install verd on Linux or macOS, check it works, and update it."},
		{"usage", "A tour of verd: the contest loop, the CLI, and the keys you use most."},
	} {
		src, err := os.ReadFile("site/content/" + p.file + ".md")
		must(err)
		body, title, toc := render(src, "")
		pg := page{Title: title, Desc: p.desc, Root: "", Body: body, TOC: toc}
		pg.Nav, pg.Footer = template.HTML(nav("", p.file)), template.HTML(footer(""))
		must(write(filepath.Join(out, p.file+".html"), proseTmpl, pg))
	}

	// docs
	var items []navItem
	titles := map[string]string{}
	bodies := map[string]string{}
	tocs := map[string][]navItem{}
	for _, d := range docOrder {
		src, err := os.ReadFile("docs/" + d.file + ".md")
		must(err)
		body, title, toc := render(src, "")
		titles[d.file], bodies[d.file], tocs[d.file] = title, body, toc // full page title; sidebar uses the short one below
		short := map[string]string{"customizing": "Customizing", "neovim": "Neovim and editors"}
		if t, ok := short[d.file]; ok {
			title = t
		}
		items = append(items, navItem{Title: title, Href: d.file + ".html", Blurb: d.blurb})
	}
	for i, d := range docOrder {
		side := make([]navItem, len(items))
		copy(side, items)
		side[i].On = true
		pg := page{Title: titles[d.file], Desc: docOrder[i].blurb, Root: "../", Body: bodies[d.file], Side: side, SideTitle: "Documentation", TOC: tocs[d.file]}
		if i > 0 {
			pg.Prev = &items[i-1]
		}
		if i < len(items)-1 {
			pg.Next = &items[i+1]
		}
		pg.Nav, pg.Footer = template.HTML(nav("../", "docs")), template.HTML(footer("../"))
		must(write(filepath.Join(out, "docs", d.file+".html"), docTmpl, pg))
	}
	idx := page{Title: "Documentation", Desc: "Guides and references for verd.", Root: "../", Side: nil}
	idx.Nav, idx.Footer = template.HTML(nav("../", "docs")), template.HTML(footer("../"))
	idx.TOC = items
	must(write(filepath.Join(out, "docs", "index.html"), docIndexTmpl, idx))
	fmt.Printf("built %d pages into %s\n", 3+len(items)+1, out)
	return nil
}

// render converts markdown to HTML: tables, heading ids, .md links -> .html, copy-able code blocks.
func render(src []byte, root string) (body, title string, toc []navItem) {
	var buf bytes.Buffer
	must(md.Convert(src, &buf))
	b := buf.String()
	if m := h1Re.FindStringSubmatch(b); m != nil {
		title = html.UnescapeString(tagRe.ReplaceAllString(m[1], ""))
	}
	for _, m := range h2Re.FindAllStringSubmatch(b, -1) {
		toc = append(toc, navItem{Title: html.UnescapeString(tagRe.ReplaceAllString(m[2], "")), Href: "#" + m[1]})
	}
	b = h2Re.ReplaceAllString(b, `<h2 id="$1">$2<a class="anchor" href="#$1">#</a></h2>`)
	b = hrefRe.ReplaceAllStringFunc(b, func(a string) string {
		m := hrefRe.FindStringSubmatch(a)
		return `href="` + fixLink(m[1]) + m[2] + `"`
	})
	return b, title, toc
}

// fixLink maps links written for GitHub (./keys.md, ./adr/..., ../README.md) onto the site.
func fixLink(h string) string {
	switch {
	case h == "", strings.Contains(h, "://"), strings.HasPrefix(h, "mailto:"):
		return h
	case strings.HasPrefix(h, "./adr"), strings.HasPrefix(h, "adr/"), strings.HasPrefix(h, "./docs/adr"):
		return repo + "/tree/main/docs/adr"
	case strings.HasSuffix(h, ".md") && (strings.HasPrefix(h, "./docs/") || strings.HasPrefix(h, "docs/")):
		return strings.TrimSuffix(filepath.Base(h), ".md") + ".html"
	case strings.HasSuffix(h, ".md") && !strings.Contains(strings.TrimPrefix(h, "./"), "/"):
		return strings.TrimSuffix(strings.TrimPrefix(h, "./"), ".md") + ".html"
	case strings.HasPrefix(h, "../"), strings.HasPrefix(h, "./"):
		return repo + "/blob/main/" + strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(h, "../"), "./"), "/")
	}
	return h
}

func nav(root, on string) string {
	l := func(key, href, label string) string {
		c := ""
		if key == on {
			c = ` class="on"`
		}
		return `<a` + c + ` href="` + root + href + `">` + label + `</a>`
	}
	return `<nav><div class="wrap"><a class="logo" href="` + root + `index.html">verd<span>_</span></a>` +
		l("home", "index.html#features", "Features") + l("install", "install.html", "Install") + l("usage", "usage.html", "Usage") +
		l("docs", "docs/index.html", "Docs") + `<a class="gh" href="` + repo + `">GitHub</a></div></nav>`
}

func footer(root string) string {
	return `<footer><div class="wrap"><span>verd · MIT licensed · not affiliated with Codeforces or CSES</span><span>` +
		`<a href="` + repo + `">GitHub</a> · <a href="` + repo + `/releases">Releases</a> · <a href="` + root + `docs/index.html">Docs</a> · ` +
		`<a href="` + repo + `/issues">Issues</a></span></div></footer>`
}

func write(path string, t *template.Template, p page) error {
	var b bytes.Buffer
	if err := t.Execute(&b, p); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

const head = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · verd</title><meta name="description" content="{{.Desc}}">
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns=%27http://www.w3.org/2000/svg%27 viewBox=%270 0 32 32%27%3E%3Crect width=%2732%27 height=%2732%27 rx=%276%27 fill=%27%2316161e%27/%3E%3Ctext x=%274%27 y=%2723%27 font-family=%27monospace%27 font-size=%2722%27 font-weight=%27bold%27 fill=%27%239ece6a%27%3Ev%3C/text%3E%3C/svg%3E">
<link rel="stylesheet" href="{{.Root}}assets/site.css"></head><body>{{.Nav}}`

const tail = `{{.Footer}}<script>
document.querySelectorAll('.prose pre').forEach(function(p){var b=document.createElement('button');b.className='copy';b.textContent='Copy';
b.onclick=function(){var t=p.querySelector('code').innerText;(navigator.clipboard?navigator.clipboard.writeText(t):Promise.reject()).then(function(){b.textContent='Copied';setTimeout(function(){b.textContent='Copy'},1500)},function(){b.textContent='Select + copy'})};p.appendChild(b)});
</script></body></html>`

const tocBlock = `{{if .TOC}}<aside class="toc"><h4>On this page</h4>{{range .TOC}}<a href="{{.Href}}">{{.Title}}</a>{{end}}</aside>{{end}}`

var proseTmpl = template.Must(template.New("prose").Parse(head + `<div class="wrap"><div class="layout nosidebar"><article class="prose">{{.BodyHTML}}</article>` + tocBlock + `</div></div>` + tail))

var docTmpl = template.Must(template.New("doc").Funcs(nil).Parse(head + `<div class="wrap"><div class="layout"><nav class="side" aria-label="Documentation"><h4>{{.SideTitle}}</h4>{{range .Side}}<a{{if .On}} class="on"{{end}} href="{{.Href}}">{{.Title}}</a>{{end}}</nav>
<article class="prose">{{.BodyHTML}}<div class="pager">{{if .Prev}}<a href="{{.Prev.Href}}"><small>Previous</small>{{.Prev.Title}}</a>{{else}}<span></span>{{end}}{{if .Next}}<a class="next" href="{{.Next.Href}}"><small>Next</small>{{.Next.Title}}</a>{{end}}</div></article>` + tocBlock + `</div></div>` + tail))

var docIndexTmpl = template.Must(template.New("idx").Parse(head + `<div class="wrap"><div class="pagehead"><div class="kicker">Documentation</div><h1>Guides and references</h1><p>Everything verd does, written to be read by people and to be followed by agents.</p></div>
<div class="docgrid">{{range .TOC}}<a class="card" href="{{.Href}}"><h3>{{.Title}}</h3><p>{{.Blurb}}</p></a>{{end}}</div></div>` + tail))

// BodyHTML is Body as trusted HTML.
func (p page) BodyHTML() template.HTML { return template.HTML(p.Body) }

var _ = sort.Strings
