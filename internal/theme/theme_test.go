package theme

import (
	"strings"
	"testing"
)

func TestThemesChangeColors(t *testing.T) {
	term, _ := Get("terminal")
	drac, ok := Get("dracula")
	if !ok {
		t.Fatal("dracula missing")
	}
	// terminal theme uses ANSI 16 indices only (inherits the terminal palette)
	if got := term.Styles(true).Good.Render("x"); !strings.Contains(got, "38;5;2mx") {
		t.Errorf("terminal good should be ANSI index 2: %q", got)
	}
	// named theme uses its own truecolor values
	if got := drac.Styles(true).Good.Render("x"); !strings.Contains(got, "80;250;123") {
		t.Errorf("dracula good should be #50fa7b: %q", got)
	}
	if term.Styles(true).Good.Render("x") == drac.Styles(true).Good.Render("x") {
		t.Error("switching theme must change rendered colors")
	}
}

func TestLightPicksLightVariants(t *testing.T) {
	for _, n := range Names() {
		th, _ := Get(n)
		if th.Styles(true).Accent.Render("x") == th.Styles(false).Accent.Render("x") && n != "terminal" {
			t.Errorf("%s: light accent must differ from dark", n)
		}
		if th.Styles(false).Glamour != "light" {
			t.Errorf("%s: light background should use glamour light style, got %q", n, th.Styles(false).Glamour)
		}
	}
	term, _ := Get("terminal")
	if term.Styles(false).Accent.Render("x") == term.Styles(true).Accent.Render("x") {
		t.Error("terminal light accent should differ (blue vs cyan)")
	}
}

func TestUnknownFallsBack(t *testing.T) {
	th, ok := Get("nope")
	if ok || th.Name != "terminal" {
		t.Fatalf("got %v %v", th.Name, ok)
	}
	if _, ok := Get(""); !ok {
		t.Fatal("empty name is the default, not an error")
	}
}
