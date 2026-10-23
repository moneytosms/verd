package scrape

import (
	"fmt"
	"html"
	"strings"
	"unicode"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

// Options configures how Render lays out and styles statements.
type Options struct {
	Width    int      // cap statement width (0 = unlimited)
	Margin   int      // left margin in columns
	Spacing  string   // compact|normal|relaxed
	Headings string   // plain|bold|bar|underline
	Math     string   // unicode|raw
	Emphasis bool     // render italics/bold
	Palette  *Palette // reading colours (nil = the theme's own)
}

// DefaultOptions returns the standard options.
func DefaultOptions() Options {
	return Options{Margin: 1, Spacing: "normal", Headings: "bar", Math: "unicode", Emphasis: true}
}

// getStyleConfig loads a glamour style by name and applies heading/margin preferences.
func getStyleConfig(name string, o Options) *ansi.StyleConfig {
	baseCfg, ok := styles.DefaultStyles[name]
	if !ok {
		return nil
	}
	cfg := *baseCfg // copy to avoid mutating the shared default
	if cfg.Document.Margin != nil {
		m := uint(o.Margin)
		cfg.Document.Margin = &m
	}
	// Apply heading styles
	applyHeadingStyle(&cfg, o.Headings)
	if !o.Emphasis { // plain text for *italic* and **bold** (some fonts render them badly)
		off := false
		cfg.Emph.Italic, cfg.Strong.Bold = &off, &off
	}
	if o.Palette != nil {
		applyPalette(&cfg, *o.Palette)
	}
	return &cfg
}

// applyHeadingStyle sets the heading display based on preference.
func applyHeadingStyle(cfg *ansi.StyleConfig, style string) {
	headings := []*ansi.StyleBlock{&cfg.H1, &cfg.H2, &cfg.H3, &cfg.H4, &cfg.H5, &cfg.H6}
	switch style {
	case "plain":
		// plain text, remove all decoration
		for _, h := range headings {
			h.Prefix = ""
			h.Suffix = ""
			if h.Bold != nil {
				*h.Bold = false
			}
		}
	case "bold":
		// bold with accent colour, no prefix
		for _, h := range headings {
			h.Prefix = ""
			h.Suffix = ""
			if h.Bold == nil {
				t := true
				h.Bold = &t
			} else {
				*h.Bold = true
			}
		}
	case "bar":
		// "▌ " prefix in accent colour + bold
		for _, h := range headings {
			h.Prefix = "▌ "
			h.Suffix = ""
			if h.Bold == nil {
				t := true
				h.Bold = &t
			} else {
				*h.Bold = true
			}
		}
	case "underline":
		// bold with a thin rule beneath (handled in post-processing)
		for _, h := range headings {
			h.Prefix = ""
			h.Suffix = "\n" + strings.Repeat("─", 40) // approximate width; real width set post-render
			if h.Bold == nil {
				t := true
				h.Bold = &t
			} else {
				*h.Bold = true
			}
		}
	}
}

// Render turns a stored statement into styled terminal text wrapped to width, using a glamour style name.
// Order matters: TeX first (html-to-markdown would escape `_` and `\`), then markdown, then glamour.
func Render(statementHTML string, width int, style string, o Options) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(statementHTML))
	if err != nil {
		return "", err
	}
	st := doc.Find(".problem-statement")
	st.Find(".header, .sample-tests").Remove() // shown separately
	st.Find(".section-title").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("<h3>" + s.Text() + "</h3>")
	})
	// HTML <sup>/<sub> (10<sup>9</sup>) would flatten to "109": use Unicode scripts like the TeX path.
	st.Find("sup").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml(html.EscapeString(script(s.Text(), supFrom, supTo, "^")))
	})
	st.Find("sub").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml(html.EscapeString(script(s.Text(), subFrom, subTo, "_")))
	})
	// Images are not text: leave a token paragraph (see below).
	// Each image becomes its own paragraph holding ImageToken(i), so the caller can draw the i-th
	// image (ImageURLs) there, or turn the token into "[image]" when it cannot.
	st.Find("img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		s.ReplaceWithHtml("<p><a href=\"" + html.EscapeString(absImage(src)) + "\">" + ImageToken(i) + "</a></p>")
	})
	// Tables (Codeforces tex-tabular) lose their cell boundaries as text: one paragraph per row instead.
	st.Find("table").Each(func(_ int, t *goquery.Selection) {
		var rows strings.Builder
		t.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			var cells []string
			tr.Find("td, th").Each(func(_ int, td *goquery.Selection) {
				cells = append(cells, strings.TrimSpace(td.Text()))
			})
			rows.WriteString("<p>" + strings.Join(cells, " ") + "</p>")
		})
		t.ReplaceWithHtml(rows.String())
	})
	body, err := st.Html()
	if err != nil {
		return "", err
	}
	// Apply math preference: skip TeX conversion if raw
	texBody := body
	if o.Math != "raw" {
		texBody = ConvertTeX(body)
	}
	markdown, err := md.ConvertString(texBody)
	if err != nil {
		return "", err
	}
	// Server text must not smuggle terminal escapes past glamour.
	markdown = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, markdown)

	// Apply spacing preference by adding/removing blank lines between blocks
	markdown = applySpacing(markdown, o.Spacing)

	// Determine render width
	renderWidth := width
	if o.Width > 0 && o.Width < width {
		renderWidth = o.Width
	}

	var rendererOpts []glamour.TermRendererOption
	if cfg := getStyleConfig(style, o); cfg != nil {
		rendererOpts = append(rendererOpts, glamour.WithStyles(*cfg))
	} else {
		rendererOpts = append(rendererOpts, glamour.WithStylePath(style))
	}
	rendererOpts = append(rendererOpts, glamour.WithWordWrap(renderWidth))

	r, err := glamour.NewTermRenderer(rendererOpts...)
	if err != nil {
		return "", err
	}
	out, err := r.Render(markdown)
	if err != nil {
		return "", err
	}

	// Center narrow content if width was capped
	if o.Width > 0 && o.Width < width {
		out = centerText(out, width, o.Width, o.Margin)
	}

	return strings.TrimRight(out, "\n"), nil
}

// applySpacing adjusts blank lines between paragraphs based on preference.
func applySpacing(markdown string, spacing string) string {
	// This is a simple approach: collapse all multiple blank lines, then add back per preference
	lines := strings.Split(markdown, "\n")
	var out []string
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
		} else {
			// Add appropriate blank lines based on spacing preference
			if blankCount > 0 {
				numBlanks := 0
				switch spacing {
				case "compact":
					numBlanks = 0
				case "normal":
					numBlanks = 1
				case "relaxed":
					numBlanks = 2
				}
				for i := 0; i < numBlanks; i++ {
					out = append(out, "")
				}
			}
			out = append(out, line)
			blankCount = 0
		}
	}
	return strings.Join(out, "\n")
}

// centerText centers narrow content inside a wider pane.
func centerText(text string, paneWidth int, contentWidth int, margin int) string {
	lines := strings.Split(text, "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		// Preserve empty lines
		if strings.TrimSpace(line) == "" {
			out[i] = ""
		} else {
			// Add margin + padding to reach pane width
			padding := (paneWidth - margin - contentWidth) / 2
			if padding < 0 {
				padding = 0
			}
			spaces := strings.Repeat(" ", margin+padding)
			out[i] = spaces + line
		}
	}
	return strings.Join(out, "\n")
}

// absImage makes an image address usable outside the page: statements store them as the site wrote
// them ("//espresso.codeforces.com/x.png", "/predownloaded/x.png", CSES "/file/<hash>").
func absImage(src string) string {
	switch {
	case strings.HasPrefix(src, "//"):
		return "https:" + src
	case strings.HasPrefix(src, "/file/"):
		return "https://cses.fi" + src
	case strings.HasPrefix(src, "/"):
		return "https://codeforces.com" + src
	}
	return src
}

// ImageToken is the text left in a rendered statement where its i-th image goes.
func ImageToken(i int) string { return fmt.Sprintf("VERDIMG%dZ", i) }

// ImageURLs are the addresses of a statement's images, in the order of their tokens.
func ImageURLs(statementHTML string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(statementHTML))
	if err != nil {
		return nil
	}
	var out []string
	doc.Find(".problem-statement").Find(".header, .sample-tests").Remove()
	doc.Find(".problem-statement img").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		out = append(out, absImage(src))
	})
	return out
}
