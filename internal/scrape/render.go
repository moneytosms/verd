package scrape

import (
	"html"
	"strings"
	"unicode"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

// getStyleConfig loads a glamour style by name (if a built-in) and removes heading prefixes for cleaner output.
// If the style name is not a built-in, returns nil to signal that a file path should be used instead.
func getStyleConfig(name string) *ansi.StyleConfig {
	baseCfg, ok := styles.DefaultStyles[name]
	if !ok {
		return nil
	}
	// Copy the config to avoid mutating the shared default.
	cfg := *baseCfg
	// Remove markdown prefixes from headings; rely on color/bold for distinction.
	cfg.H1.Prefix = ""
	cfg.H2.Prefix = ""
	cfg.H3.Prefix = ""
	cfg.H4.Prefix = ""
	cfg.H5.Prefix = ""
	cfg.H6.Prefix = ""
	// Reduce document margin for compact rendering.
	if cfg.Document.Margin != nil {
		margin := uint(1)
		cfg.Document.Margin = &margin
	}
	return &cfg
}

// Render turns a stored statement into styled terminal text wrapped to width, using a glamour style name.
// Order matters: TeX first (html-to-markdown would escape `_` and `\`), then markdown, then glamour.
func Render(statementHTML string, width int, style string) (string, error) {
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
	// Images can't render in a terminal: replace with alt text + OSC 8 link.
	st.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		s.ReplaceWithHtml("<a href=\"" + src + "\">[image]</a>")
	})
	body, err := st.Html()
	if err != nil {
		return "", err
	}
	markdown, err := md.ConvertString(ConvertTeX(body))
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

	// Try to load as a built-in style; fall back to file path if not found.
	var rendererOpts []glamour.TermRendererOption
	if cfg := getStyleConfig(style); cfg != nil {
		rendererOpts = append(rendererOpts, glamour.WithStyles(*cfg))
	} else {
		rendererOpts = append(rendererOpts, glamour.WithStylePath(style))
	}
	rendererOpts = append(rendererOpts, glamour.WithWordWrap(width))

	r, err := glamour.NewTermRenderer(rendererOpts...)
	if err != nil {
		return "", err
	}
	out, err := r.Render(markdown)
	return strings.TrimRight(out, "\n"), err
}
