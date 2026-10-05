package scrape

import (
	"os"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	b, _ := os.ReadFile("testdata/1167B.html")
	d, _ := Parse(b)
	out, err := Render(d.Statement, 80, "dark")
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
	out, _ = Render(d.Statement, 80, "dark")
	if !strings.Contains(out, "espresso.codeforces.com") || strings.Contains(out, "Example") {
		t.Errorf("image link missing or samples not stripped:\n%s", out)
	}
}

func TestRenderSuperAndSubscripts(t *testing.T) {
	out, err := Render(`<div class="problem-statement"><p>n &le; 10<sup class="upper-index">9</sup>, a<sub class="lower-index">i</sub>, 2<sup>k+1</sup></p></div>`, 80, "notty")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"10⁹", "aᵢ", "2ᵏ⁺¹"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}
