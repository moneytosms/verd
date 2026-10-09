package scrape

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRender(t *testing.T) {
	b, _ := os.ReadFile("testdata/1167B.html")
	d, _ := Parse(b)
	out, err := Render(d.Statement, 80, "dark", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "interactive problem") || !strings.Contains(out, "Interaction") {
		t.Errorf("missing text:\n%s", out)
	}
	if strings.Contains(out, "$$$") || strings.Contains(out, `\le`) {
		t.Errorf("TeX leaked:\n%s", out)
	}
	if strings.Contains(out, "Lost Numbers") || strings.Contains(out, "time limit per test") {
		t.Errorf("header should be stripped:\n%s", out)
	}
	b, _ = os.ReadFile("testdata/1900A.html")
	d, _ = Parse(b)
	out, _ = Render(d.Statement, 80, "dark", DefaultOptions())
	if !strings.Contains(out, "espresso.codeforces.com") || strings.Contains(out, "Example") {
		t.Errorf("image link missing or samples not stripped:\n%s", out)
	}
}

func TestRenderSuperAndSubscripts(t *testing.T) {
	out, err := Render(`<div class="problem-statement"><p>n &le; 10<sup class="upper-index">9</sup>, a<sub class="lower-index">i</sub>, 2<sup>k+1</sup></p></div>`, 80, "notty", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"10⁹", "aᵢ", "2ᵏ⁺¹"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestRenderNoHeadingMarkers(t *testing.T) {
	html := `<div class="problem-statement">
	<div class="section-title">Input</div>
	<p>First line of input.</p>
	<div class="section-title">Output</div>
	<p>First line of output.</p>
	<div class="section-title">Note</div>
	<p>Some notes about the problem.</p>
	<pre>1 2 3</pre>
	</div>`

	for _, styleName := range []string{"dark", "light", "notty"} {
		out, err := Render(html, 80, styleName, DefaultOptions())
		if err != nil {
			t.Fatalf("style %s: %v", styleName, err)
		}

		// Check no lines start with "#" or "###" (markdown heading syntax)
		for i, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
				t.Errorf("style %s: line %d starts with markdown heading marker: %q", styleName, i, line)
			}
		}

		// Check heading text is present and not wrapped in markdown syntax
		if !strings.Contains(out, "Input") {
			t.Errorf("style %s: missing 'Input' heading text", styleName)
		}
		if !strings.Contains(out, "Output") {
			t.Errorf("style %s: missing 'Output' heading text", styleName)
		}

		// Check line width constraint
		for i, line := range strings.Split(out, "\n") {
			width := ansi.StringWidth(line)
			if width > 80 {
				t.Errorf("style %s: line %d exceeds width 80 (got %d): %q", styleName, i, width, line)
			}
		}
	}
}

func TestRenderMathWithoutForcedItalics(t *testing.T) {
	html := `<div class="problem-statement">
	<p>Constraint: 1 &le; n &le; 10<sup>9</sup></p>
	<div class="section-title">Examples</div>
	<p>For a₁ = 2 and a₂ = 3, answer is 5.</p>
	</div>`

	out, err := Render(html, 80, "dark", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	// Check Unicode math renders (not LaTeX escapes)
	if !strings.Contains(out, "≤") || !strings.Contains(out, "10⁹") {
		t.Errorf("Unicode math not rendered correctly:\n%s", out)
	}

	// Check Examples section appears
	if !strings.Contains(out, "Examples") {
		t.Errorf("'Examples' section missing")
	}
}

func TestRenderHeadingsBar(t *testing.T) {
	html := `<div class="problem-statement"><h3>Input</h3><p>Test</p></div>`
	o := DefaultOptions()
	o.Headings = "bar"
	out, err := Render(html, 80, "notty", o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "▌") {
		t.Errorf("missing bar heading marker in %q", out)
	}
}

func TestRenderHeadingsUnderline(t *testing.T) {
	html := `<div class="problem-statement"><h3>Input</h3><p>Test</p></div>`
	o := DefaultOptions()
	o.Headings = "underline"
	out, err := Render(html, 80, "notty", o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "─") {
		t.Errorf("missing underline in heading %q", out)
	}
}

func TestRenderMathRaw(t *testing.T) {
	html := `<div class="problem-statement"><p>$x = 2^{10}$</p></div>`
	o := DefaultOptions()
	o.Math = "raw"
	out, err := Render(html, 80, "notty", o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "^") {
		t.Errorf("raw math should preserve TeX backslash: %q", out)
	}
}

func TestRenderWidth(t *testing.T) {
	html := `<div class="problem-statement"><p>This is a very long line that should be capped at a specific width when the width option is set.</p></div>`
	o := DefaultOptions()
	o.Width = 40
	out, err := Render(html, 100, "notty", o)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check rendered content width (content without padding)
		if len(trimmed) > 45 { // allow some wiggle room for rendering artifacts
			t.Errorf("line content too long with width cap: %q (%d chars)", trimmed, len(trimmed))
		}
	}
}

func TestRenderEmphasisOff(t *testing.T) {
	html := `<div class="problem-statement"><p><b>bold</b> and <i>italic</i> text</p></div>`
	o := DefaultOptions()
	o.Emphasis = false
	out, err := Render(html, 80, "notty", o)
	if err != nil {
		t.Fatal(err)
	}
	// Should not have bold/italic SGR codes
	if strings.Contains(out, "\x1b[1m") || strings.Contains(out, "\x1b[3m") {
		t.Errorf("emphasis SGR codes found when emphasis is off: %q", out)
	}
}
