package scrape

import (
	"strings"
	"unicode"

	"charm.land/glamour/v2"
	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

// Render turns a stored statement into styled terminal text wrapped to width.
// Order matters: TeX first (html-to-markdown would escape `_` and `\`), then markdown, then glamour.
func Render(statementHTML string, width int) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(statementHTML))
	if err != nil {
		return "", err
	}
	st := doc.Find(".problem-statement")
	st.Find(".header, .sample-tests").Remove() // shown separately
	st.Find(".section-title").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("<h3>" + s.Text() + "</h3>")
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
	r, err := glamour.NewTermRenderer(glamour.WithStylePath("dark"), glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	out, err := r.Render(markdown)
	return strings.TrimRight(out, "\n"), err
}
