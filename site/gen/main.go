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

const (
	repo    = "https://github.com/moneytosms/verd"
	siteURL = "https://moneytosms.github.io/verd/" // social cards need absolute image URLs
)

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
	{"contests", "Virtual, practice and live contest sessions: timer, scoring, hidden ratings."},
	{"editorials", "Where editorials come from, hint-by-hint reveal and offline use."},
}

// guideOrder is the Guide chapters in reading order (docs/guide/<file>.md); title is the sidebar label.
var guideOrder = []struct{ file, title, blurb string }{
	{"start", "Quick start", "Install, set up and solve your first problem in five minutes."},
	{"problems", "Finding problems", "Search, filters, the Picker, stats, CSES and working offline."},
	{"solve", "Read, edit and test", "The Problem view, editors, test runs, verdicts, comparison modes and custom tests."},
	{"stress", "Stress testing", "Find the input where your solution and a brute force disagree, and make it fast."},
	{"submit", "Submitting", "Browser handoff and direct mode, and following the verdict."},
	{"contests", "Contests and sessions", "Virtual, practice and live sessions with a timer and scoring."},
	{"editorials", "Editorials", "Read an editorial one hint at a time."},
	{"snippets", "Snippets and Templates", "Starter files and a library of ready algorithms: bfs, dijkstra, modpow and more."},
	{"customize", "Customize", "Config, themes, shortcuts, reading, layout and your own languages."},
	{"cli", "Command line", "Every command, exit codes and scripting examples."},
	{"editors", "Neovim and editors", "Splits, the embedded pane, keymaps and snippets in your editor."},
	{"troubleshooting", "Troubleshooting", "The doctor, common errors and where verd keeps its files."},
}

type navItem struct {
	Title, Href, Blurb string
	On                 bool
}

type page struct {
	Title, Desc, Root, Body string
	Nav                     template.HTML
	Footer                  template.HTML
	HeadX, Tail             template.HTML
	Side                    []navItem
	SideTitle               string
	TOC                     []navItem
	Prev, Next              *navItem
	Heading, Lead           string // heading of an index page
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
	s := strings.NewReplacer("{{ROOT}}", "", "{{HEADX}}", headX(""), "{{NAV}}", nav("", "home"), "{{FOOTER}}", footer(""), "{{TAIL}}", tailHTML("")).Replace(string(home))
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
		decorate(&pg, "", p.file)
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
		decorate(&pg, "../", "docs")
		must(write(filepath.Join(out, "docs", d.file+".html"), docTmpl, pg))
	}
	idx := page{Title: "Documentation", Desc: "Guides and references for verd.", Root: "../", Side: nil,
		Heading: "References", Lead: "Every option, key and file, written to be read by people and to be followed by agents. New here? Start with the Guide."}
	decorate(&idx, "../", "docs")
	idx.TOC = items
	must(write(filepath.Join(out, "docs", "index.html"), docIndexTmpl, idx))

	// guide
	must(os.MkdirAll(filepath.Join(out, "guide"), 0o755))
	var gitems []navItem
	gbodies := map[string]string{}
	gtocs := map[string][]navItem{}
	gtitles := map[string]string{}
	for _, g := range guideOrder {
		src, err := os.ReadFile("docs/guide/" + g.file + ".md")
		must(err)
		body, title, toc := renderIn(src, "guide")
		gbodies[g.file], gtocs[g.file], gtitles[g.file] = body, toc, title
		gitems = append(gitems, navItem{Title: g.title, Href: g.file + ".html", Blurb: g.blurb})
	}
	for i, g := range guideOrder {
		side := make([]navItem, len(gitems))
		copy(side, gitems)
		side[i].On = true
		pg := page{Title: gtitles[g.file], Desc: g.blurb, Root: "../", Body: gbodies[g.file], Side: side, SideTitle: "Guide", TOC: gtocs[g.file]}
		if i > 0 {
			pg.Prev = &gitems[i-1]
		}
		if i < len(gitems)-1 {
			pg.Next = &gitems[i+1]
		}
		decorate(&pg, "../", "guide")
		must(write(filepath.Join(out, "guide", g.file+".html"), docTmpl, pg))
	}
	gidx := page{Title: "Guide", Desc: "A step-by-step guide to everything verd does, with small examples.", Root: "../",
		Heading: "Learn verd, step by step", Lead: "Short chapters with examples you can copy. Read them in order, or jump to the one you need. For every option and key, see the References."}
	decorate(&gidx, "../", "guide")
	gidx.TOC = gitems
	must(write(filepath.Join(out, "guide", "index.html"), docIndexTmpl, gidx))
	fmt.Printf("built %d pages into %s\n", 3+len(items)+1+len(gitems)+1, out)
	return nil
}

// render converts markdown to HTML: tables, heading ids, .md links -> .html, copy-able code blocks.
func render(src []byte, root string) (body, title string, toc []navItem) {
	return renderIn(src, "")
}

// renderIn is render for a page of a section ("" = docs, "guide"): links are written for GitHub, where
// docs/guide/x.md links to ../config.md, and are mapped onto the site layout.
func renderIn(src []byte, section string) (body, title string, toc []navItem) {
	var buf bytes.Buffer
	must(md.Convert(src, &buf))
	b := buf.String()
	if m := h1Re.FindStringSubmatch(b); m != nil {
		title = html.UnescapeString(tagRe.ReplaceAllString(m[1], ""))
	}
	for _, m := range h2Re.FindAllStringSubmatch(b, -1) {
		toc = append(toc, navItem{Title: html.UnescapeString(tagRe.ReplaceAllString(m[2], "")), Href: "#" + m[1]})
	}
	b = highlightCode(b)
	b = h2Re.ReplaceAllString(b, `<h2 id="$1">$2<a class="anchor" href="#$1">#</a></h2>`)
	b = hrefRe.ReplaceAllStringFunc(b, func(a string) string {
		m := hrefRe.FindStringSubmatch(a)
		return `href="` + fixLink(m[1], section) + m[2] + `"`
	})
	return b, title, toc
}

// fixLink maps links written for GitHub (./keys.md, ./adr/..., ../README.md) onto the site.
func fixLink(h, section string) string {
	switch {
	case h == "", strings.Contains(h, "://"), strings.HasPrefix(h, "mailto:"):
		return h
	case section == "guide" && strings.HasSuffix(h, ".md"):
		return guideLink(h)
	case strings.HasSuffix(h, ".md") && (strings.HasPrefix(h, "./guide/") || strings.HasPrefix(h, "guide/")):
		return "../guide/" + strings.TrimSuffix(filepath.Base(h), ".md") + ".html"
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

// guideLink maps a .md link inside a guide chapter to the page it became.
func guideLink(h string) string {
	base := strings.TrimSuffix(filepath.Base(h), ".md")
	if !strings.HasPrefix(h, "../") {
		return base + ".html" // ./other-chapter.md
	}
	// docs/guide/x.md -> docs/<base>.md; install and usage are site pages
	if base == "install" || base == "usage" {
		return "../" + base + ".html"
	}
	return "../docs/" + base + ".html"
}

func decorate(p *page, root, on string) {
	p.Nav, p.Footer = template.HTML(nav(root, on)), template.HTML(footer(root))
	p.HeadX, p.Tail = template.HTML(headX(root)), template.HTML(tailHTML(root))
}

// headX is the shared <head> tail: icons, social card, fonts and the stylesheet. The "js" class lets
// the stylesheet hide things for an entrance only when the script that reveals them is running.
func headX(root string) string {
	return `<link rel="icon" type="image/svg+xml" href="` + root + `assets/favicon.svg"><link rel="apple-touch-icon" href="` + root + `assets/favicon.svg">` +
		`<meta property="og:title" content="verd: competitive programming in your terminal"><meta property="og:image" content="` + siteURL + `assets/launch.jpg"><meta name="twitter:card" content="summary_large_image">` +
		`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin><link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&family=Space+Grotesk:wght@400;500;600&display=swap">` +
		`<link rel="stylesheet" href="` + root + `assets/site.css"><script>document.documentElement.className="js"</script>`
}

// tailHTML loads the script every page shares: the bar, entrances, copy buttons and scroll position.
func tailHTML(root string) string {
	return `<script src="` + root + `assets/site.js" defer></script>`
}

// nav is the bar: the wordmark, the places to go (the current one marked), and the one action.
func nav(root, on string) string {
	l := func(key, href, label string) string {
		c := ""
		if key == on {
			c = ` class="on"`
		}
		return `<a` + c + ` href="` + root + href + `">` + label + `</a>`
	}
	return `<header class="bar" id="bar"><a class="logo" href="` + root + `index.html"><img src="` + root + `assets/favicon.svg" alt="" width="28" height="28">verd</a>` +
		`<nav class="links" id="menu" aria-label="Site">` + l("how", "index.html#how", "How it works") + l("features", "index.html#features", "Features") +
		l("guide", "guide/index.html", "Guide") + l("docs", "docs/index.html", "Reference") + l("install", "install.html", "Install") + `</nav>` +
		`<div class="acts"><a class="gh" href="` + repo + `">GitHub</a><a class="key blue" href="` + root + `index.html#install">Get verd</a>` +
		`<button class="key burger" type="button" aria-expanded="false" aria-controls="menu">Menu</button></div><span class="prog"></span></header>`
}

// footer closes every page with the name built from four keys.
func footer(root string) string {
	return `<footer><div class="mark" aria-hidden="true"><span>v</span><span>e</span><span>r</span><span>d</span></div><div class="foot">` +
		`<div><a href="` + root + `guide/index.html">Guide</a><a href="` + root + `docs/index.html">Docs</a><a href="` + root + `usage.html">Usage</a></div>` +
		`<div><a href="` + root + `install.html">Install</a><a href="` + repo + `/releases">Releases</a><a href="` + root + `docs/keys.html">Keys</a></div>` +
		`<div><a href="` + repo + `">GitHub</a><a href="` + repo + `/issues">Issues</a><a href="` + root + `guide/troubleshooting.html">Troubleshooting</a></div>` +
		`<div class="wide"><span>verd is a keyboard-driven terminal app for Codeforces and CSES.</span><span class="soft">MIT licensed. Not affiliated with Codeforces or CSES.</span></div></div>` +
		`<div class="last"><span>Made for people who would rather not alt+tab.</span><a class="key blue" href="` + root + `index.html#install">Install verd</a></div></footer>`
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
<title>{{.Title}} · verd</title><meta name="description" content="{{.Desc}}"><meta name="theme-color" content="#0f1012">
{{.HeadX}}</head><body class="docs">{{.Nav}}`

const tail = `{{.Footer}}{{.Tail}}</body></html>`

const tocBlock = `{{if .TOC}}<aside class="toc"><h4>On this page</h4>{{range .TOC}}<a href="{{.Href}}">{{.Title}}</a>{{end}}</aside>{{end}}`

var proseTmpl = template.Must(template.New("prose").Parse(head + `<div class="layout nosidebar"><article class="prose">{{.BodyHTML}}</article>` + tocBlock + `</div>` + tail))

var docTmpl = template.Must(template.New("doc").Funcs(nil).Parse(head + `<div class="layout"><nav class="side" aria-label="Documentation"><h4>{{.SideTitle}}</h4>{{range .Side}}<a{{if .On}} class="on"{{end}} href="{{.Href}}">{{.Title}}</a>{{end}}</nav>
<article class="prose">{{.BodyHTML}}<div class="pager">{{if .Prev}}<a href="{{.Prev.Href}}"><small>Previous</small>{{.Prev.Title}}</a>{{else}}<span></span>{{end}}{{if .Next}}<a class="next" href="{{.Next.Href}}"><small>Next</small>{{.Next.Title}}</a>{{end}}</div></article>` + tocBlock + `</div>` + tail))

var docIndexTmpl = template.Must(template.New("idx").Parse(head + `<div class="pagehead"><h1>{{.Heading}}</h1><p>{{.Lead}}</p></div>
<div class="docgrid">{{range $c := .TOC}}<a href="{{$c.Href}}"><h3>{{$c.Title}}</h3><p>{{$c.Blurb}}</p></a>{{end}}</div>` + tail))

// BodyHTML is Body as trusted HTML.
func (p page) BodyHTML() template.HTML { return template.HTML(p.Body) }

var _ = sort.Strings
