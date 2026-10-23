package scrape

import (
	"strings"
	"testing"
)

func TestPaletteChangesColours(t *testing.T) {
	html := `<div class="problem-statement"><p>Hello <b>bold</b></p></div>`
	plain, err := Render(html, 60, "dark", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "48;2;") || strings.Contains(plain, "38;2;") {
		t.Fatalf("nil palette should keep the theme's colours: %q", plain)
	}
	p, ok := PaletteNamed("paper")
	if !ok {
		t.Fatal("paper palette missing")
	}
	o := DefaultOptions()
	o.Palette = &p
	out, err := Render(html, 60, "dark", o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "48;2;247;241;227") || !strings.Contains(out, "38;2;58;50;38") {
		t.Fatalf("paper colours missing: %q", out)
	}
	if _, ok := PaletteNamed("app"); ok {
		t.Fatal("app is the theme, not a palette")
	}
}
