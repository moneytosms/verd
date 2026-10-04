// Package scrape parses Codeforces problem pages into statement, limits and Sample Tests.
package scrape

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var ErrNoStatement = errors.New("no problem statement on page")

type Sample struct{ Input, Output string }

type Detail struct {
	Statement     string // raw HTML of .problem-statement
	TimeLimitMS   int
	MemoryLimitMB int
	Interactive   bool
	Hint          string // Comparison Mode hint: "float" or ""
	Samples       []Sample
}

var (
	floatHint = regexp.MustCompile(`(?i)absolute or relative error|absolute error|relative error`)
	limitNum  = regexp.MustCompile(`[\d.]+`)
)

// Parse extracts a Detail from a problem page.
func Parse(page []byte) (*Detail, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	st := doc.Find(".problem-statement").First()
	if st.Length() == 0 {
		return nil, ErrNoStatement
	}
	html, err := goquery.OuterHtml(st)
	if err != nil {
		return nil, err
	}
	d := &Detail{Statement: html}

	d.TimeLimitMS = int(limit(st, ".time-limit") * 1000)
	d.MemoryLimitMB = int(limit(st, ".memory-limit"))

	text := st.Text()
	d.Interactive = strings.Contains(text, "interactive problem") || st.Find(".section-title").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return strings.TrimSpace(s.Text()) == "Interaction"
	}).Length() > 0
	if floatHint.MatchString(text) {
		d.Hint = "float"
	}

	var ins, outs []string
	st.Find(".sample-test .input pre").Each(func(_ int, s *goquery.Selection) { ins = append(ins, preText(s)) })
	st.Find(".sample-test .output pre").Each(func(_ int, s *goquery.Selection) { outs = append(outs, preText(s)) })
	if len(ins) != len(outs) {
		return nil, fmt.Errorf("sample mismatch: %d inputs, %d outputs", len(ins), len(outs))
	}
	for i := range ins {
		d.Samples = append(d.Samples, Sample{ins[i], outs[i]})
	}
	return d, nil
}

// limit reads the first number from a header property, e.g. "2 seconds" or "256 megabytes".
func limit(st *goquery.Selection, sel string) float64 {
	var v float64
	fmt.Sscanf(limitNum.FindString(st.Find(sel).First().Clone().Children().Remove().End().Text()), "%g", &v)
	return v
}

// preText handles both sample formats: per-line <div>s (new) and <br>-separated text (old).
func preText(pre *goquery.Selection) string {
	var s string
	if lines := pre.Find(".test-example-line"); lines.Length() > 0 {
		var parts []string
		lines.Each(func(_ int, l *goquery.Selection) { parts = append(parts, l.Text()) })
		s = strings.Join(parts, "\n")
	} else {
		c := pre.Clone()
		c.Find("br").ReplaceWithHtml("\n")
		s = c.Text()
	}
	s = strings.ReplaceAll(s, "\r", "")
	return strings.Trim(s, "\n") + "\n"
}
