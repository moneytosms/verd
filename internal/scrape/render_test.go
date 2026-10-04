package scrape

import (
	"os"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	b, _ := os.ReadFile("testdata/1167B.html")
	d, _ := Parse(b)
	out, err := Render(d.Statement, 80)
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
	out, _ = Render(d.Statement, 80)
	if !strings.Contains(out, "espresso.codeforces.com") || strings.Contains(out, "Example") {
		t.Errorf("image link missing or samples not stripped:\n%s", out)
	}
}
